package sat

// Source: src/sat/sat_types.h:153-204.
// Concrete elimination todo, model-converter and integrity-check variable sets.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type satSolverUintSet struct {
	self       *satSolverUintSet
	inSet      []byte // source svector<char>, false0/true1.
	values     []uint32
	budget     *arena.Budget
	ownerBytes uint64
	closed     bool
}
type satSolverUintSetStats struct {
	OwnerBytes, MembershipSize, MembershipCapacity, ValuesSize, ValuesCapacity uint64
}

func newSATSolverUintSet(b *arena.Budget) (*satSolverUintSet, error) {
	n := uint64(reflect.TypeOf(satSolverUintSet{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &satSolverUintSet{budget: b, ownerBytes: n}
	s.self = s
	return s, nil
}
func (s *satSolverUintSet) open() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *satSolverUintSet) Size() uint32 {
	if s == nil || s.closed {
		return 0
	}
	return uint32(len(s.values))
}
func (s *satSolverUintSet) Empty() bool { return s.Size() == 0 }
func (s *satSolverUintSet) MembershipSize() uint64 {
	if s == nil || s.closed {
		return 0
	}
	return uint64(len(s.inSet))
}
func (s *satSolverUintSet) MembershipCapacity() uint64 {
	if s == nil || s.closed {
		return 0
	}
	return uint64(cap(s.inSet))
}
func (s *satSolverUintSet) ValuesCapacity() uint64 {
	if s == nil || s.closed {
		return 0
	}
	return uint64(cap(s.values))
}
func (s *satSolverUintSet) Stats() satSolverUintSetStats {
	return satSolverUintSetStats{s.ownerBytes, s.MembershipSize(), s.MembershipCapacity(), uint64(s.Size()), s.ValuesCapacity()}
}
func (s *satSolverUintSet) Contains(v uint32) bool {
	return s != nil && s.self == s && !s.closed && uint64(v) < uint64(len(s.inSet)) && s.inSet[v] != 0
}
func (s *satSolverUintSet) Insert(v uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	if v == ^uint32(0) {
		return arena.ErrBounds
	}
	n := int(v) + 1
	if n > len(s.inSet) {
		s.inSet = append(s.inSet, make([]byte, n-len(s.inSet))...)
	}
	if s.inSet[v] == 0 {
		s.inSet[v] = 1
		s.values = append(s.values, v)
	}
	return nil
}
func (s *satSolverUintSet) Reset() error {
	if e := s.open(); e != nil {
		return e
	}
	s.values = s.values[:0]
	s.inSet = s.inSet[:0]
	return nil
}
func (s *satSolverUintSet) clearBacking() { s.values = nil; s.inSet = nil }
func (s *satSolverUintSet) Finalize() error {
	if e := s.open(); e != nil {
		return e
	}
	s.clearBacking()
	return nil
}
func (s *satSolverUintSet) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.clearBacking()
	s.budget.Release(s.ownerBytes)
	s.ownerBytes = 0
	s.closed = true
	return nil
}
