package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:349-379.

import "github.com/glycerine/g3/pkg/sat"

func (s *subsumptionSimplifier) unitSubsumption(source, target *Clause) (bool, error) {
	if source.Size() != 1 {
		return false, nil
	}
	unit := source.Literals()[0]
	for _, literal := range target.Literals() {
		subsumes, err := s.literalSubsumes(unit, literal.Negated())
		if err != nil {
			return false, err
		}
		if !subsumes {
			continue
		}
		replacement := make([]sat.Literal, 0, target.Size()-1)
		for _, current := range target.Literals() {
			if current != literal {
				replacement = append(replacement, current)
			}
		}
		if len(replacement) == 0 {
			return false, nil
		}
		assumptions, err := s.solver.assumptions.mkJoin(target.Assumptions(), source.Assumptions())
		if err != nil {
			return false, err
		}
		clause, err := s.solver.clauses.mkClause(replacement, false, assumptions)
		if err != nil {
			return false, err
		}
		if err = s.computeOccurrences(clause); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
