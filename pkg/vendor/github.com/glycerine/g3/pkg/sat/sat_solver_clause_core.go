package sat

// Source: src/sat/sat_solver.cpp:367–465; src/sat/sat_solver.h value/lvl accessors.

import (
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Borrowed views of the solver's literal-indexed assignment and variable-indexed
// level arrays. This cold view neither owns nor copies those arrays. The solver
// supplies its actual scope level on each call; it is not a search implementation.
type satSolverClauseState struct {
	variables  *satSolverVariableState
	traceTags  *diagnostic.TraceTags
	assignment []int8
	levels     []uint32
	sorter     satLiteralOrder
}

func (s *satSolverClauseState) value(l Literal) (lbool.LBool, error) {
	if s == nil {
		return lbool.Undef, ErrSATOwner
	}
	if s.variables != nil {
		return s.variables.valueLiteral(l)
	}
	if uint64(l.index()) >= uint64(len(s.assignment)) {
		return lbool.Undef, arena.ErrBounds
	}
	return lbool.LBool(s.assignment[l.index()]), nil
}
func (s *satSolverClauseState) level(l Literal) (uint32, error) {
	if s == nil {
		return 0, ErrSATOwner
	}
	if s.variables != nil {
		return s.variables.levelVar(l.variable())
	}
	if uint64(l.variable()) >= uint64(len(s.levels)) {
		return 0, arena.ErrBounds
	}
	return s.levels[l.variable()], nil
}

// Native source-order sort over the captured literal slice. Equal literal words
// carry identical values, so stability is not observable. The caller keeps
// this concrete adapter in its existing state to reuse the interface box.
type satLiteralOrder struct{ values []Literal }

func (a *satLiteralOrder) Len() int           { return len(a.values) }
func (a *satLiteralOrder) Less(i, j int) bool { return a.values[i] < a.values[j] }
func (a *satLiteralOrder) Swap(i, j int)      { a.values[i], a.values[j] = a.values[j], a.values[i] }

func (a *satLiteralOrder) sort(values []Literal) {
	a.values = values
	sort.Sort(a)
	a.values = nil
}

func (s *satSolverClauseState) simplifyClause(scope uint32, n *uint32, lits []Literal) (bool, error) {
	if s == nil {
		return false, ErrSATOwner
	}
	if n == nil || uint64(*n) > uint64(len(lits)) {
		return false, arena.ErrBounds
	}
	return s.simplifyClauseSpan(scope, n, satSolverLiteralSpan{literals: lits})
}
func (s *satSolverClauseState) simplifyClauseSpan(scope uint32, n *uint32, span satSolverLiteralSpan) (bool, error) {
	if s == nil {
		return false, ErrSATOwner
	}
	if n == nil {
		return false, arena.ErrBounds
	}
	if span.column == nil && span.store == nil && uint64(*n) > uint64(len(span.literals)) {
		return false, arena.ErrBounds
	}
	if e := satSortClauseSpan(span, *n, &s.sorter); e != nil {
		return false, e
	}
	prev := DefaultLiteral()
	var j uint32
	for i := uint32(0); i < *n; i++ {
		curr, e := span.literal(i)
		if e != nil {
			return false, e
		}
		val, e := s.value(curr)
		if e != nil {
			return false, e
		}
		if scope != 0 {
			lvl, e := s.level(curr)
			if e != nil {
				return false, e
			}
			if lvl > 0 {
				val = lbool.Undef
			}
		}
		switch val {
		case lbool.False:
		case lbool.Undef:
			if curr == prev.Negated() {
				return false, nil
			}
			if curr != prev {
				prev = curr
				if i != j {
					l, e := span.literal(i)
					if e != nil {
						return false, e
					}
					if e = span.setLiteral(j, l); e != nil {
						return false, e
					}
				}
				j++
			}
		case lbool.True:
			return false, nil
		}
	}
	*n = j
	return true, nil
}

func (s *satSolverClauseState) selectWatchLiteral(a *ClauseAllocator, clause StoredClauseRef, start uint32) (uint32, error) {
	if s == nil {
		return 0, ErrSATOwner
	}
	n, e := a.Size(clause)
	if e != nil {
		return 0, e
	}
	if satStorageSourceDebug && n < 2 {
		return 0, ErrSATAssertion
	}
	none := ^uint32(0)
	minTrue, maxFalse, unknown := none, none, none
	for i := start; i < n; i++ {
		l, e := a.Literal(clause, i)
		if e != nil {
			return 0, e
		}
		val, e := s.value(l)
		if e != nil {
			return 0, e
		}
		switch val {
		case lbool.False:
			if maxFalse == none {
				maxFalse = i
			} else {
				current, e := s.level(l)
				if e != nil {
					return 0, e
				}
				previous, e := a.Literal(clause, maxFalse)
				if e != nil {
					return 0, e
				}
				level, e := s.level(previous)
				if e != nil {
					return 0, e
				}
				if current > level {
					maxFalse = i
				}
			}
		case lbool.Undef:
			unknown = i
		case lbool.True:
			if minTrue == none {
				minTrue = i
			} else {
				current, e := s.level(l)
				if e != nil {
					return 0, e
				}
				previous, e := a.Literal(clause, minTrue)
				if e != nil {
					return 0, e
				}
				level, e := s.level(previous)
				if e != nil {
					return 0, e
				}
				if current < level {
					minTrue = i
				}
			}
		}
	}
	if minTrue != none {
		return minTrue, nil
	}
	if unknown != none {
		return unknown, nil
	}
	if satStorageSourceDebug && maxFalse == none {
		return 0, ErrSATAssertion
	}
	return maxFalse, nil
}

func (s *satSolverClauseState) selectLearnedWatchLiteral(a *ClauseAllocator, clause StoredClauseRef) (uint32, error) {
	if s == nil {
		return 0, ErrSATOwner
	}
	n, e := a.Size(clause)
	if e != nil {
		return 0, e
	}
	if satStorageSourceDebug && n < 2 {
		return 0, ErrSATAssertion
	}
	maxFalse := ^uint32(0)
	for i := uint32(1); i < n; i++ {
		l, e := a.Literal(clause, i)
		if e != nil {
			return 0, e
		}
		if e = s.traceLearnedWatchValue(l); e != nil {
			return 0, e
		}
		if satStorageSourceDebug {
			val, e := s.value(l)
			if e != nil {
				return 0, e
			}
			if val != lbool.False {
				return 0, ErrSATAssertion
			}
		}
		if maxFalse == ^uint32(0) {
			maxFalse = i
		} else {
			current, e := s.level(l)
			if e != nil {
				return 0, e
			}
			previous, e := a.Literal(clause, maxFalse)
			if e != nil {
				return 0, e
			}
			level, e := s.level(previous)
			if e != nil {
				return 0, e
			}
			if current > level {
				maxFalse = i
			}
		}
	}
	return maxFalse, nil
}
