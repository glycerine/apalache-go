package euclid

// Source: src/math/euclid/euclidean_solver.cpp:224-284,575-622.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *solverSetup) betterPivot(absA *numeral.Int, x uint32, eqSize int) (bool, error) {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return false, e
	}
	if enabled {
		positive, e := s.manager.Integers().IsPos(absA)
		if e != nil {
			return false, e
		}
		if !positive {
			if e := diagnostic.DebugAssertionFailure(d, "src/math/euclid/euclidean_solver.cpp", 226, "m().is_pos(a)"); e != nil {
				return false, e
			}
		}
	}
	if x >= uint32(len(s.occurrences)) {
		return false, arena.ErrBounds
	}
	if s.nextX == nullEquationIndex {
		return true, nil
	}
	cmp, e := s.manager.Integers().Compare(absA, &s.nextA)
	if e != nil || cmp != 0 {
		return cmp < 0, e
	}
	if len(s.occurrences[x]) != len(s.occurrences[s.nextX]) {
		return len(s.occurrences[x]) < len(s.occurrences[s.nextX]), nil
	}
	return eqSize < len(s.equations[s.nextEq].xs), nil
}

func (s *solverSetup) updateNextCandidate(eqIndex uint32) (err error) {
	eq := s.equations[eqIndex]
	if eq == nil {
		return nil
	}
	z := s.manager.Integers()
	absA := numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&absA)) }()
	for i := range eq.xs {
		if err = z.Copy(&absA, &eq.as[i]); err != nil {
			return err
		}
		if err = z.Abs(&absA); err != nil {
			return err
		}
		better, e := s.betterPivot(&absA, eq.xs[i], len(eq.xs))
		if e != nil {
			return e
		}
		if !better {
			continue
		}
		if err = z.Copy(&s.nextA, &absA); err != nil {
			return err
		}
		s.nextX, s.nextEq = eq.xs[i], eqIndex
		if s.nextPosA, err = z.IsPos(&eq.as[i]); err != nil {
			return err
		}
	}
	return nil
}

// selectNextVar scans source equation slots from the end, taking the smallest
// absolute coefficient, then fewer occurrences, then the shorter equation.
// A unit coefficient ends the scan immediately.
func (s *solverSetup) selectNextVar() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	for len(s.equations) != 0 && s.equations[len(s.equations)-1] == nil {
		s.equations = s.equations[:len(s.equations)-1]
	}
	if len(s.equations) == 0 {
		return false, nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return false, e
	}
	if enabled && (len(s.equations) == 0 || s.equations[len(s.equations)-1] == nil) {
		if e := diagnostic.DebugAssertionFailure(d, "src/math/euclid/euclidean_solver.cpp", 275, "!m_equations.empty() && m_equations.back() != 0"); e != nil {
			return false, e
		}
	}
	s.nextX = nullEquationIndex
	for i := len(s.equations) - 1; i >= 0; i-- {
		if e := s.updateNextCandidate(uint32(i)); e != nil {
			return false, e
		}
		if s.nextX != nullEquationIndex {
			one, e := s.manager.Integers().IsOne(&s.nextA)
			if e != nil {
				return false, e
			}
			if one {
				return true, nil
			}
		}
	}
	if e := euclidSourceAssertion(287, "m_next_x != null_var", s.nextX != nullEquationIndex); e != nil {
		return false, e
	}
	if s.nextX == nullEquationIndex {
		return false, arena.ErrInvalidSlot
	}
	return true, nil
}

func (s *solverSetup) eraseOccurrence(x, eqIndex uint32) error {
	if x >= uint32(len(s.occurrences)) {
		return arena.ErrBounds
	}
	s.occurrences[x] = eraseOccurrenceValue(s.occurrences[x], eqIndex)
	return nil
}

func (s *solverSetup) substituteMostRecentSolution(x uint32) error {
	if len(s.solution) == 0 || x >= uint32(len(s.occurrences)) {
		return arena.ErrInvalidSlot
	}
	if e := euclidSourceAssertion(582, "!m_solution.empty()", len(s.solution) != 0); e != nil {
		return e
	}
	if e := s.traceMostRecentSolution(x, s.solution[len(s.solution)-1]); e != nil {
		return e
	}
	// The source iterates x's original use-list while each substitution may
	// update other variables' lists. The exception x stays until the loop ends.
	for _, eqIndex := range s.occurrences[x] {
		if eqIndex >= uint32(len(s.equations)) || s.equations[eqIndex] == nil {
			return arena.ErrInvalidSlot
		}
		eq := s.equations[eqIndex]
		if e := s.traceSubstitutionEquation(x, eq, false); e != nil {
			return e
		}
		if e := s.applySolutionOnce(x, eq, true, false, eqIndex, x, nil); e != nil {
			return e
		}
		if e := s.traceSubstitutionEquation(x, eq, true); e != nil {
			return e
		}
		if e := s.normalizeEquation(eqIndex); e != nil {
			return e
		}
		if s.inconsistent != nullEquationIndex {
			break
		}
	}
	s.occurrences[x] = nil
	return nil
}

// eliminateUnit is the source elim_unit branch after select_next_var. A
// positive pivot is negated so the promoted equation solves -x plus its
// remaining terms; occurrences are removed before solution publication.
func (s *solverSetup) eliminateUnit() error {
	if e := s.open(); e != nil {
		return e
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	z := s.manager.Integers()
	one, e := z.IsOne(&s.nextA)
	if e != nil {
		return e
	}
	if enabled && !one {
		if e := diagnostic.DebugAssertionFailure(d, "src/math/euclid/euclidean_solver.cpp", 603, "m().is_one(m_next_a)"); e != nil {
			return e
		}
	}
	if s.nextX == nullEquationIndex || s.nextEq >= uint32(len(s.equations)) ||
		s.equations[s.nextEq] == nil {
		return arena.ErrInvalidSlot
	}
	if !one {
		return arena.ErrInvalidSlot
	}
	eq := s.equations[s.nextEq]
	if e := s.traceUnitElimination(eq); e != nil {
		return e
	}
	if s.nextPosA {
		for i := range eq.as {
			if e := z.Neg(&eq.as[i]); e != nil {
				return e
			}
		}
		for i := range eq.bs {
			if e := s.manager.Neg(&eq.bs[i]); e != nil {
				return e
			}
		}
		if e := z.Neg(&eq.c); e != nil {
			return e
		}
	}
	for _, x := range eq.xs {
		if e := s.eraseOccurrence(x, s.nextEq); e != nil {
			return e
		}
	}
	if e := s.promoteSolvedEquation(s.nextEq, s.nextX); e != nil {
		return e
	}
	return s.substituteMostRecentSolution(s.nextX)
}
