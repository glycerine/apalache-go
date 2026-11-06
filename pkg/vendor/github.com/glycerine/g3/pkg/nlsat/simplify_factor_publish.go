package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:157-240, cleanup_removed.

import "github.com/glycerine/g3/pkg/sat"

func (s *factorSimplifier) publish(candidate factorSimplificationCandidate, replacements [][]sat.Literal) error {
	for _, literals := range replacements {
		if _, err := s.solver.clauses.mkClause(literals, false, candidate.clause.Assumptions()); err != nil {
			return err
		}
	}
	return s.solver.clauses.deleteClause(candidate.clause)
}
