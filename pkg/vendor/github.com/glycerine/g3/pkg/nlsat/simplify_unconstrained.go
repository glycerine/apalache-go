package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:246-302; operator() loop at 39-40.

type unconstrainedEqualitySimplifier struct {
	solver  *Solver
	removed map[*Clause]bool
}

// SimplifyUnconstrainedEqualities removes clauses made true by assigning a
// variable that occurs in no other clause. It iterates to the source fixed
// point and retains reverse model patches for every eliminated variable.
func (s *Solver) SimplifyUnconstrainedEqualities() (count uint32, supported bool, err error) {
	if err = s.open(); err != nil {
		return 0, false, err
	}
	if err = s.search.initializeSearch(); err != nil {
		return 0, false, err
	}
	root, err := s.originalClausesHaveRootAtom()
	if err != nil || root {
		return 0, !root, err
	}
	phase := &unconstrainedEqualitySimplifier{solver: s}
	for {
		removed, cause := phase.eliminatePass()
		if cause != nil {
			return count, true, cause
		}
		count += removed
		if removed == 0 {
			return count, true, nil
		}
	}
}

func (s *unconstrainedEqualitySimplifier) eliminatePass() (uint32, error) {
	occurrences, err := s.variableOccurrences()
	if err != nil {
		return 0, err
	}
	s.removed = make(map[*Clause]bool)
	var count uint32
	for variable := len(occurrences); variable > 0; variable-- {
		clauses := occurrences[variable-1]
		if len(clauses) != 1 || s.removed[clauses[0]] {
			continue
		}
		matched, cause := s.eliminateFromClause(uint32(variable-1), clauses[0])
		if cause != nil {
			return count, cause
		}
		if matched {
			s.removed[clauses[0]] = true
			count++
			s.solver.search.simplifications++
		}
	}
	for clause := range s.removed {
		if err = s.solver.clauses.deleteClause(clause); err != nil {
			return count, err
		}
	}
	return count, nil
}
