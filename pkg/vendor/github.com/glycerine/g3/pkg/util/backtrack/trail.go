package backtrack

// Source: src/util/trail.h:25-357.

import (
	"errors"
	"reflect"
)

var (
	ErrTrailNil    = errors.New("g3: nil trail")
	ErrTrailScopes = errors.New("g3: trail scope underflow")
)

// Trail is one reversible operation. Concrete Go consumers normally use
// TrailFunc so captured typed state replaces the source template subclasses.
type Trail interface {
	Undo()
}

// TrailFunc adapts a closure to Trail.
type TrailFunc func()

func (f TrailFunc) Undo() { f() }

func nilTrail(trail Trail) bool {
	if trail == nil {
		return true
	}
	value := reflect.ValueOf(trail)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// UndoTrailStack undoes entries [oldSize, len(trails)) in reverse order and
// returns the retained prefix.
func UndoTrailStack(trails []Trail, oldSize uint32) ([]Trail, error) {
	if uint64(oldSize) > uint64(len(trails)) {
		return trails, ErrTrailScopes
	}
	for i := len(trails); i > int(oldSize); i-- {
		trails[i-1].Undo()
		trails[i-1] = nil
	}
	return trails[:oldSize], nil
}

// TrailStack owns reversible operations and nested scope boundaries.
// Go closures replace the source's templated trail subclasses and its region
// allocator; captured values remain alive through ordinary Go reachability.
type TrailStack struct {
	trails []Trail
	scopes []uint32
}

func NewTrailStack() *TrailStack { return &TrailStack{} }

// PushPtr retains a trail object until it is undone or Reset.
func (s *TrailStack) PushPtr(trail Trail) error {
	if nilTrail(trail) {
		return ErrTrailNil
	}
	s.trails = append(s.trails, trail)
	return nil
}

// Push is the Go counterpart of the source value-copying push. Interface
// values and closures have Go value semantics and require no arena copy.
func (s *TrailStack) Push(trail Trail) error { return s.PushPtr(trail) }

// PushUndo captures a concrete undo closure.
func (s *TrailStack) PushUndo(undo func()) error {
	return s.Push(TrailFunc(undo))
}

func (s *TrailStack) GetNumScopes() uint32 { return uint32(len(s.scopes)) }

func (s *TrailStack) PushScope() {
	s.scopes = append(s.scopes, uint32(len(s.trails)))
}

// PopScope undoes numScopes nested scopes, newest first.
func (s *TrailStack) PopScope(numScopes uint32) error {
	if numScopes == 0 {
		return nil
	}
	if uint64(numScopes) > uint64(len(s.scopes)) {
		return ErrTrailScopes
	}
	newLevel := len(s.scopes) - int(numScopes)
	oldSize := s.scopes[newLevel]
	var err error
	s.trails, err = UndoTrailStack(s.trails, oldSize)
	if err != nil {
		return err
	}
	clear(s.scopes[newLevel:])
	s.scopes = s.scopes[:newLevel]
	return nil
}

// Reset pops all scopes and then undoes operations stored at level zero.
func (s *TrailStack) Reset() error {
	if err := s.PopScope(uint32(len(s.scopes))); err != nil {
		return err
	}
	var err error
	s.trails, err = UndoTrailStack(s.trails, 0)
	return err
}
