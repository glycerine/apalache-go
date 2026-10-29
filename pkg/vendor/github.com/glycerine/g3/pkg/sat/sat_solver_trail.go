package sat

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// The solver's ordered assignment trail owns native literal backing. Its
// checked root identity remains stable across growth and shrink; free and close
// retire it. Only the actual cold owner header is budgeted.
type satSolverTrail struct {
	self               *satSolverTrail
	values             []Literal
	budget             *arena.Budget
	origin, ownerBytes uint64
	live, closed       bool
}

func newSATSolverTrail(b *arena.Budget) (*satSolverTrail, satSolverLiteralListRef, error) {
	if b == nil {
		return nil, satSolverLiteralListRef{}, ErrSATOwner
	}
	n := uint64(reflect.TypeOf(satSolverTrail{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, satSolverLiteralListRef{}, e
	}
	id, e := satNextOwner()
	if e != nil {
		b.Release(n)
		return nil, satSolverLiteralListRef{}, e
	}
	t := &satSolverTrail{budget: b, origin: id, ownerBytes: n, live: true}
	t.self = t
	return t, satSolverLiteralListRef{owner: id, generation: 1}, nil
}
func (t *satSolverTrail) open() error {
	if t == nil || t.self != t {
		return ErrSATOwner
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *satSolverTrail) check(r satSolverLiteralListRef) error {
	if e := t.open(); e != nil {
		return e
	}
	if r.owner != t.origin {
		return ErrSATOwner
	}
	if !t.live || r.slot != 0 || r.generation != 1 {
		return arena.ErrInvalidSlot
	}
	return nil
}
func (t *satSolverTrail) Size(r satSolverLiteralListRef) (uint32, error) {
	if e := t.check(r); e != nil {
		return 0, e
	}
	return uint32(len(t.values)), nil
}
func (t *satSolverTrail) Push(r satSolverLiteralListRef, l Literal) error {
	if e := t.check(r); e != nil {
		return e
	}
	if uint64(len(t.values)) >= uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	t.values = append(t.values, l)
	return nil
}
func (t *satSolverTrail) Read(r satSolverLiteralListRef, i uint32) (Literal, error) {
	if e := t.check(r); e != nil {
		return 0, e
	}
	if uint64(i) >= uint64(len(t.values)) {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return t.values[i], nil
}
func (t *satSolverTrail) Shrink(r satSolverLiteralListRef, n uint32) error {
	if e := t.check(r); e != nil {
		return e
	}
	if uint64(n) > uint64(len(t.values)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	t.values = t.values[:n]
	return nil
}
func (t *satSolverTrail) IsLive(r satSolverLiteralListRef) bool { return t.check(r) == nil }
func (t *satSolverTrail) Free(r satSolverLiteralListRef) error {
	if e := t.check(r); e != nil {
		return e
	}
	t.values = nil
	t.live = false
	return nil
}
func (t *satSolverTrail) Close() error {
	if t == nil || t.self != t {
		return ErrSATOwner
	}
	if t.closed {
		return nil
	}
	t.values = nil
	t.live = false
	t.budget.Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
