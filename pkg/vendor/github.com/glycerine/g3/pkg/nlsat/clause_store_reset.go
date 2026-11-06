package nlsat

// Source: src/nlsat/nlsat_solver.cpp:645-654.

func (s *clauseStore) resetClauses() error {
	if err := s.open(); err != nil {
		return err
	}
	for len(s.originals) != 0 || len(s.learned) != 0 {
		clauses := s.originals
		if len(clauses) == 0 {
			clauses = s.learned
		}
		if err := s.deleteClause(clauses[0]); err != nil {
			return err
		}
	}
	return nil
}
