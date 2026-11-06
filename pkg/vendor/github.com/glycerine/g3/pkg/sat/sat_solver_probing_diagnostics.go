package sat

// Source: src/sat/sat_probing.cpp; src/sat/sat_probing.h.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Actual source cpp3022-3074 conditional check producers, backed by real clause
// owners and assignment columns. They do not replace integrity_checker.

func (s *satSolverClauseLifecycle) clauseUnit(c StoredClauseRef) (bool, error) {
	p := s.propagation
	n, e := p.clauses.Size(c)
	if e != nil {
		return false, e
	}
	found := false
	for i := uint32(0); i < n; i++ {
		l, e := p.clauses.Literal(c, i)
		if e != nil {
			return false, e
		}
		v, e := p.variables.valueLiteral(l)
		if e != nil {
			return false, e
		}
		switch v {
		case lbool.Undef:
			if found {
				return false, nil
			}
			found = true
		case lbool.True:
			return false, nil
		case lbool.False:
		}
	}
	return found, nil
}
func (s *satSolverClauseLifecycle) clauseEmpty(c StoredClauseRef) (bool, error) {
	p := s.propagation
	n, e := p.clauses.Size(c)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		l, e := p.clauses.Literal(c, i)
		if e != nil {
			return false, e
		}
		v, e := p.variables.valueLiteral(l)
		if e != nil {
			return false, e
		}
		if v != lbool.False {
			return false, nil
		}
	}
	return true, nil
}
func (s *satSolverClauseLifecycle) checkMissedClauseVector(cs []StoredClauseRef) error {
	// Source captures begin/end; concrete source calls have no owner mutation.
	for i := 0; i < len(cs); i++ {
		c := cs[i]
		frozen, e := s.propagation.clauses.Frozen(c)
		if e != nil {
			return e
		}
		if frozen {
			continue
		}
		empty, e := s.clauseEmpty(c)
		if e != nil {
			return e
		}
		unit := false
		if !empty {
			unit, e = s.clauseUnit(c)
			if e != nil {
				return e
			}
		}
		if empty || unit {
			s.traceMissedPropagation(c)
			if !satStorageSourceDebug {
				return ErrSATAssertion
			}
			d := diagnostic.BorrowProcessDebugContext()
			if !d.HasAssertionActions() {
				return ErrSATAssertion
			}
			if e = d.UnreachableSource("src/sat/sat_solver.cpp", 3063); e != nil {
				return e
			}
		}
		if satStorageSourceDebug {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, e := d.AssertionsEnabled()
			if e != nil {
				return e
			}
			if enabled {
				empty, e = s.clauseEmpty(c)
				if e != nil {
					return e
				}
				if empty {
					if !d.HasAssertionActions() {
						return ErrSATAssertion
					}
					if e = diagnostic.DebugAssertionFailure(d, "src/sat/sat_solver.cpp", 3065, "!is_empty(c)"); e != nil {
						return e
					}
				}
			}
			enabled, e = d.AssertionsEnabled()
			if e != nil {
				return e
			}
			if enabled {
				unit, e = s.clauseUnit(c)
				if e != nil {
					return e
				}
				if unit {
					if !d.HasAssertionActions() {
						return ErrSATAssertion
					}
					if e = diagnostic.DebugAssertionFailure(d, "src/sat/sat_solver.cpp", 3066, "!is_unit(c)"); e != nil {
						return e
					}
				}
			}
		}
	}
	return nil
}
func (s *satSolverClauseLifecycle) checkMissedPropagation() error {
	if e := s.open(); e != nil {
		return e
	}
	if s.propagation.inconsistent {
		return nil
	}
	if e := s.checkMissedClauseVector(s.Original); e != nil {
		return e
	}
	return s.checkMissedClauseVector(s.learned)
}

func (s *satSolverProbing) clauseUnit(c StoredClauseRef) (bool, error) {
	return s.lifecycle.clauseUnit(c)
}
func (s *satSolverProbing) clauseEmpty(c StoredClauseRef) (bool, error) {
	return s.lifecycle.clauseEmpty(c)
}
func (s *satSolverProbing) checkMissedClauseVector(cs []StoredClauseRef) error {
	return s.lifecycle.checkMissedClauseVector(cs)
}
func (s *satSolverProbing) checkMissedPropagation() error {
	return s.lifecycle.checkMissedPropagation()
}
