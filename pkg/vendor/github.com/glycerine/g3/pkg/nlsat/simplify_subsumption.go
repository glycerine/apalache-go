package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:304-478.

type subsumptionSimplifier struct {
	solver      *Solver
	occurrences [][]*Clause
	hashes      map[*Clause]uint32
	marked      map[*Clause]bool
	removed     map[*Clause]bool
}

// SimplifySubsumedClauses runs the retained occurrence-indexed subsumption and
// unit-subsumption-resolution phase over original clauses.
func (s *Solver) SimplifySubsumedClauses() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if err := s.search.initializeSearch(); err != nil {
		return 0, err
	}
	phase := &subsumptionSimplifier{
		solver: s, hashes: make(map[*Clause]uint32), marked: make(map[*Clause]bool), removed: make(map[*Clause]bool),
	}
	return phase.run()
}

func (s *subsumptionSimplifier) run() (uint32, error) {
	for _, clause := range append([]*Clause(nil), s.solver.clauses.originals...) {
		if err := s.computeOccurrences(clause); err != nil {
			return 0, err
		}
	}
	var count uint32
	for variable := len(s.occurrences); variable > 0; variable-- {
		clauses := s.occurrences[variable-1]
		size := len(clauses)
		for i := 0; i < size; i++ {
			source := clauses[i]
			if s.marked[source] || s.removed[source] {
				continue
			}
			s.marked[source] = true
			for j := 0; j < size; j++ {
				target := clauses[j]
				if source == target || s.removed[target] {
					continue
				}
				subsumes, err := s.clauseSubsumes(source, target)
				if err != nil {
					return count, err
				}
				if !subsumes {
					subsumes, err = s.unitSubsumption(source, target)
					if err != nil {
						return count, err
					}
				}
				if subsumes {
					s.removed[target] = true
					count++
					s.solver.search.simplifications++
				}
			}
		}
	}
	if err := s.cleanupRemoved(); err != nil {
		return count, err
	}
	return count, nil
}
