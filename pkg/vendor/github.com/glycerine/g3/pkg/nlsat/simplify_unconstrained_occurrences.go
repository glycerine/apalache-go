package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:251-263,304-335.

func (s *unconstrainedEqualitySimplifier) variableOccurrences() ([][]*Clause, error) {
	collector := &subsumptionSimplifier{
		solver: s.solver, hashes: make(map[*Clause]uint32), marked: make(map[*Clause]bool), removed: make(map[*Clause]bool),
	}
	for _, clause := range s.solver.clauses.originals {
		if err := collector.computeOccurrences(clause); err != nil {
			return nil, err
		}
	}
	return collector.occurrences, nil
}
