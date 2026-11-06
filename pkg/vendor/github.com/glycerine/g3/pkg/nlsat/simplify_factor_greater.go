package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:205-233.

import "github.com/glycerine/g3/pkg/sat"

func (s *factorSimplifier) greaterReplacement(candidate factorSimplificationCandidate) ([][]sat.Literal, error) {
	literals, err := s.relationLiterals(candidate, false)
	if err != nil {
		return nil, err
	}
	if candidate.literal.Sign() {
		// p*q <= 0: (!(p > 0) or !(q > 0)) and
		//             (!(q < 0) or !(p < 0)).
		return [][]sat.Literal{
			{literals.great[0].Negated(), literals.great[1].Negated()},
			{literals.less[1].Negated(), literals.less[0].Negated()},
		}, nil
	}
	// p*q > 0: (p < 0 or q > 0) and (q < 0 or p > 0).
	return [][]sat.Literal{
		{literals.less[0], literals.great[1]},
		{literals.less[1], literals.great[0]},
	}, nil
}
