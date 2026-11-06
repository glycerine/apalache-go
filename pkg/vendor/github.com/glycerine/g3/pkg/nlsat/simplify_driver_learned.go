package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:29-34.

func (s *Solver) removeSimplifierLearnedClauses() (uint32, error) {
	learned := append([]*Clause(nil), s.clauses.learned...)
	for i, clause := range learned {
		if err := s.clauses.deleteClause(clause); err != nil {
			return uint32(i), err
		}
	}
	return uint32(len(learned)), nil
}
