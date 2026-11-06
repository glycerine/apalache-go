package polynomial

// Source: src/math/polynomial/polynomial.cpp:6218-6232,6328-6344.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// The source reference member is borrowed. These charged cold adapters never
// own or copy the helper vectors or the captured caller variable backing.
type ScopedPolynomialMaxDegrees struct {
	self                                       *ScopedPolynomialMaxDegrees
	owner                                      *PolynomialMaxDegrees
	header                                     uint64
	constructed, finalizing, finalized, closed bool
	sourceCause                                error
}

func newScopedPolynomialMaxDegrees(owner *PolynomialMaxDegrees) (*ScopedPolynomialMaxDegrees, error) {
	if e := owner.open(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(ScopedPolynomialMaxDegrees{}).Size())
	if e := owner.runtime.Budget().Reserve(h); e != nil {
		return nil, e
	}
	s := &ScopedPolynomialMaxDegrees{owner: owner, header: h}
	s.self = s
	return s, nil
}
func NewScopedPolynomialMaxDegrees(owner *PolynomialMaxDegrees, p Polynomial) (*ScopedPolynomialMaxDegrees, error) {
	s, e := newScopedPolynomialMaxDegrees(owner)
	if e != nil {
		return nil, e
	}
	if e = owner.Init(p); e != nil {
		owner.runtime.Budget().Release(s.header)
		s.header = 0
		s.closed = true
		return nil, e
	}
	s.constructed = true
	return s, nil
}
func (s *ScopedPolynomialMaxDegrees) identity() error {
	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *ScopedPolynomialMaxDegrees) open() error {
	if e := s.identity(); e != nil {
		return e
	}
	if !s.constructed {
		return ErrPolynomialPending
	}
	if s.finalizing {
		return arena.ErrClosed
	}
	return s.owner.open()
}
func (s *ScopedPolynomialMaxDegrees) Degree(x uint32) (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.owner.Degree(x)
}
func (s *ScopedPolynomialMaxDegrees) NumVariables() (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.owner.NumVariables()
}
func (s *ScopedPolynomialMaxDegrees) Vars() (PolynomialVariablesView, error) {
	if e := s.open(); e != nil {
		return PolynomialVariablesView{}, e
	}
	return s.owner.Vars()
}
func (s *ScopedPolynomialMaxDegrees) SourceFinalize() error {
	if e := s.identity(); e != nil {
		return e
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if s.finalized {
		return nil
	}
	s.finalizing = true
	if s.constructed {
		if e := s.owner.Reset(); e != nil {
			s.sourceCause = e
			return e
		}
	}
	s.finalized = true
	return nil
}
func (s *ScopedPolynomialMaxDegrees) CloseChecked() error {
	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.closed {
		return nil
	}
	if !s.finalizing {
		if e := s.SourceFinalize(); e != nil {
			return e
		}
	} // explicit owning abort only releases this adapter after a source stop.
	s.owner.runtime.Budget().Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
func NewScopedPolynomialMaxDegreesCopy(source *ScopedPolynomialMaxDegrees) (*ScopedPolynomialMaxDegrees, error) {
	if e := source.open(); e != nil {
		return nil, e
	}
	s, e := newScopedPolynomialMaxDegrees(source.owner)
	if e != nil {
		return nil, e
	}
	s.constructed = true
	return s, nil
}

// A user-declared source destructor suppresses the implicit move constructor;
// an rvalue binds to the implicit copy constructor, copying only the reference.
func NewScopedPolynomialMaxDegreesMove(source *ScopedPolynomialMaxDegrees) (*ScopedPolynomialMaxDegrees, error) {
	return NewScopedPolynomialMaxDegreesCopy(source)
}

// Source implicit assignment is deleted because of the reference member.
type ScopedPolynomialVariablePositions struct {
	self                                       *ScopedPolynomialVariablePositions
	owner                                      *PolynomialVariablePositions
	variables                                  PolynomialVariablesView
	count                                      uint32
	header                                     uint64
	constructed, finalizing, finalized, closed bool
	sourceCause                                error
}

func newScopedPolynomialVariablePositions(owner *PolynomialVariablePositions, n uint32, xs PolynomialVariablesView) (*ScopedPolynomialVariablePositions, error) {
	if e := owner.open(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(ScopedPolynomialVariablePositions{}).Size())
	if e := owner.runtime.Budget().Reserve(h); e != nil {
		return nil, e
	}
	s := &ScopedPolynomialVariablePositions{owner: owner, variables: xs, count: n, header: h}
	s.self = s
	return s, nil
}
func NewScopedPolynomialVariablePositions(owner *PolynomialVariablePositions, n uint32, xs PolynomialVariablesView) (*ScopedPolynomialVariablePositions, error) {
	s, e := newScopedPolynomialVariablePositions(owner, n, xs)
	if e != nil {
		return nil, e
	}
	if e = owner.Init(s.count, s.variables); e != nil {
		owner.runtime.Budget().Release(s.header)
		s.header = 0
		s.closed = true
		return nil, e
	}
	s.constructed = true
	return s, nil
}
func (s *ScopedPolynomialVariablePositions) identity() error {
	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *ScopedPolynomialVariablePositions) open() error {
	if e := s.identity(); e != nil {
		return e
	}
	if !s.constructed {
		return ErrPolynomialPending
	}
	if s.finalizing {
		return arena.ErrClosed
	}
	return s.owner.open()
}
func (s *ScopedPolynomialVariablePositions) Position(x uint32) (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.owner.Position(x)
}
func (s *ScopedPolynomialVariablePositions) SourceFinalize() error {
	if e := s.identity(); e != nil {
		return e
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if s.finalized {
		return nil
	}
	s.finalizing = true
	if s.constructed {
		if e := s.owner.Reset(s.count, s.variables); e != nil {
			s.sourceCause = e
			return e
		}
	}
	s.finalized = true
	return nil
}
func (s *ScopedPolynomialVariablePositions) CloseChecked() error {
	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.closed {
		return nil
	}
	if !s.finalizing {
		if e := s.SourceFinalize(); e != nil {
			return e
		}
	}
	s.owner.runtime.Budget().Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
func NewScopedPolynomialVariablePositionsCopy(source *ScopedPolynomialVariablePositions) (*ScopedPolynomialVariablePositions, error) {
	if e := source.open(); e != nil {
		return nil, e
	}
	s, e := newScopedPolynomialVariablePositions(source.owner, source.count, source.variables)
	if e != nil {
		return nil, e
	}
	s.constructed = true
	return s, nil
}
func NewScopedPolynomialVariablePositionsMove(source *ScopedPolynomialVariablePositions) (*ScopedPolynomialVariablePositions, error) {
	return NewScopedPolynomialVariablePositionsCopy(source)
}
