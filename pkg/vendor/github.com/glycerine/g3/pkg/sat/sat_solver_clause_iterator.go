package sat

// Source: src/sat/sat_solver.h:539-542; src/util/vector.h:290-294.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// Captured begin/end pointers are represented by a borrowed backing and end.
// Existing pushClause replaces a backing ONLY with strictly larger capacity.
// Thus capacity equality proves identity within this owner lifetime; same-cap
// append/compaction remains visible. This does not implement source pointer8
// allocation ABI for the existing Go24 clause references.
type satSolverClauseIterator struct {
	owner      *Solver
	backing    []StoredClauseRef
	index, end uint32
	capacity   int
	learned    bool
}

func (s *Solver) beginOriginalClauses() (satSolverClauseIterator, error) {
	return s.beginClauseIterator(false)
}
func (s *Solver) beginLearnedClauses() (satSolverClauseIterator, error) {
	return s.beginClauseIterator(true)
}
func (s *Solver) beginClauseIterator(learned bool) (satSolverClauseIterator, error) {
	if e := s.open(); e != nil {
		return satSolverClauseIterator{}, e
	}
	xs := s.Lifecycle.Original
	if learned {
		xs = s.Lifecycle.learned
	}
	return satSolverClauseIterator{owner: s, backing: xs[:cap(xs)], end: uint32(len(xs)), capacity: cap(xs), learned: learned}, nil
}
func (it *satSolverClauseIterator) Next() (StoredClauseRef, bool, error) {
	if it == nil {
		return StoredClauseRef{}, false, ErrSATOwner
	}
	if e := it.owner.open(); e != nil {
		return StoredClauseRef{}, false, e
	}
	if it.index >= it.end {
		return StoredClauseRef{}, false, nil
	}
	xs := it.owner.Lifecycle.Original
	if it.learned {
		xs = it.owner.Lifecycle.learned
	}
	if cap(xs) != it.capacity {
		return StoredClauseRef{}, false, arena.ErrBounds
	}
	if uint64(it.index) >= uint64(len(it.backing)) {
		return StoredClauseRef{}, false, arena.ErrBounds
	}
	c := it.backing[it.index]
	it.index++
	return c, true, nil
}
