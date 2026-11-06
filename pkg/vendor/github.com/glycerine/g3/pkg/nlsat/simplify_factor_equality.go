package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:163-180.

import "github.com/glycerine/g3/pkg/sat"

func (s *factorSimplifier) equalityReplacement(candidate factorSimplificationCandidate) ([][]sat.Literal, error) {
	literals, err := s.relationLiterals(candidate, true)
	if err != nil {
		return nil, err
	}
	if candidate.literal.Sign() {
		return [][]sat.Literal{{literals.equal[0].Negated()}, {literals.equal[1].Negated()}}, nil
	}
	return [][]sat.Literal{{literals.equal[0], literals.equal[1]}}, nil
}
