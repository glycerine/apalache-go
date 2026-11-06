package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:336-347.

func (s *subsumptionSimplifier) cleanupRemoved() error {
	for clause := range s.removed {
		if err := s.solver.clauses.deleteClause(clause); err != nil {
			return err
		}
	}
	return nil
}
