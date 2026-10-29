package euclid

// Source: src/math/euclid/euclidean_solver.cpp:708-730.

// solve follows the source pivot loop. A contradiction from either an earlier
// assertion or a substitution returns false while retaining the explaining
// equation in the todo list.
func (s *solverSetup) solve() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if s.inconsistent != nullEquationIndex {
		return false, nil
	}
	if e := s.traceSolveEntry(); e != nil {
		s.stop = e
		return false, e
	}
	z := s.manager.Integers()
	for {
		found, e := s.selectNextVar()
		if e != nil {
			s.stop = e
			return false, e
		}
		if !found {
			return true, nil
		}
		if e = s.traceSolveCandidate(); e != nil {
			s.stop = e
			return false, e
		}
		one, e := z.IsOne(&s.nextA)
		if e != nil {
			s.stop = e
			return false, e
		}
		minusOne, e := z.IsMinusOne(&s.nextA)
		if e != nil {
			s.stop = e
			return false, e
		}
		if one || minusOne {
			e = s.eliminateUnit()
		} else {
			e = s.decomposeAndEliminate()
		}
		if e != nil {
			s.stop = e
			return false, e
		}
		if e = s.traceSolveAfter(); e != nil {
			s.stop = e
			return false, e
		}
		if s.inconsistent != nullEquationIndex {
			return false, nil
		}
	}
}
