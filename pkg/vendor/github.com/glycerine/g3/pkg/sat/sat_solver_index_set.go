package sat

// Concrete insertion-ordered indices for the retained SAT consequence algorithm.

import "github.com/glycerine/g3/pkg/arena"

func (s *satSolverIndexSet) open() error {
	if s == nil {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *satSolverIndexSet) invalidate() {
	if s.view != nil {
		s.view.valid = false
		s.view = nil
	}
}
func (s *satSolverIndexSet) contains(k uint32) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	_, yes := s.keys.Get2(k)
	return yes, nil
}
func (s *satSolverIndexSet) insert(k uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	if _, yes := s.keys.Get2(k); !yes {
		s.invalidate()
		s.keys.Put(k, struct{}{})
	}
	return nil
}
func (s *satSolverIndexSet) reset() error {
	if e := s.open(); e != nil {
		return e
	}
	s.invalidate()
	s.keys.Clear()
	return nil
}
func (s *satSolverIndexSet) assign(other *satSolverIndexSet) error {
	if e := s.open(); e != nil {
		return e
	}
	if e := other.open(); e != nil {
		return e
	}
	if s == other {
		return nil
	}
	s.reset()
	for k := range other.keys.All() {
		s.keys.Put(k, struct{}{})
	}
	return nil
}
func (s *satSolverIndexSet) union(other *satSolverIndexSet) error {
	if e := s.open(); e != nil {
		return e
	}
	if e := other.open(); e != nil {
		return e
	}
	if s == other {
		return nil
	}
	for k := range other.keys.All() {
		if e := s.insert(k); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSolverIndexSet) close() error {
	if s == nil {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.invalidate()
	s.keys.Clear()
	s.closed = true
	return nil
}

type satSolverIndexValue struct {
	value   uint32
	present bool
}
type satSolverIndexView struct {
	keys  []uint32
	valid bool
}
type satSolverIndexIterator struct {
	view  *satSolverIndexView
	index int
}

// Iterators borrow one ordered view until keys change, reset or close occurs.
// Duplicate insertions leave that logical view valid, regardless of backing.
func (s *satSolverIndexSet) capture() (satSolverIndexIterator, error) {
	if e := s.open(); e != nil {
		return satSolverIndexIterator{}, e
	}
	if s.view == nil {
		s.view = &satSolverIndexView{valid: true}
		for k := range s.keys.All() {
			s.view.keys = append(s.view.keys, k)
		}
	}
	return satSolverIndexIterator{view: s.view}, nil
}
func (it *satSolverIndexIterator) next() (satSolverIndexValue, error) {
	if it == nil || it.view == nil || !it.view.valid {
		return satSolverIndexValue{}, ErrSATOwner
	}
	if it.index == len(it.view.keys) {
		return satSolverIndexValue{}, nil
	}
	v := it.view.keys[it.index]
	it.index++
	return satSolverIndexValue{v, true}, nil
}
