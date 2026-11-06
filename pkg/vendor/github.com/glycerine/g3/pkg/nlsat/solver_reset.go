package nlsat

// Source: src/nlsat/nlsat_solver.cpp:222-230,3076-3078.

// Reset removes the current clauses, learned state, arithmetic atoms, cached
// polynomials, and model while preserving declared arithmetic/plain Boolean
// variables, parameters, and cumulative statistics. This is the source
// solver::reset lifecycle, not ResetStatistics.
func (s *Solver) Reset() error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.closeFourierMotzkinBounds(); err != nil {
		return err
	}
	if err := s.explanation.reset(); err != nil {
		return err
	}
	if err := s.resolution.reset(); err != nil {
		return err
	}
	if err := s.search.resetForSolver(); err != nil {
		return err
	}
	if err := s.resetAtomReferences(); err != nil {
		return err
	}
	if err := s.clauses.resetClauses(); err != nil {
		return err
	}
	if err := s.atoms.deleteUnreferenced(); err != nil {
		return err
	}
	if err := s.cache.Reset(); err != nil {
		return err
	}
	return s.assignment.Reset()
}
