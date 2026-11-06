package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:152-161.

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

type factorRelationLiterals struct {
	equal [2]sat.Literal
	less  [2]sat.Literal
	great [2]sat.Literal
}

func (s *factorSimplifier) literal(kind AtomKind, candidate factorSimplificationCandidate, factor int) (sat.Literal, error) {
	return s.solver.atoms.mkInequalityLiteral(kind,
		[]polynomial.Polynomial{candidate.polynomial[factor]},
		[]bool{candidate.even[factor]})
}

func (s *factorSimplifier) relationLiterals(candidate factorSimplificationCandidate, equality bool) (factorRelationLiterals, error) {
	var result factorRelationLiterals
	for factor := 0; factor < 2; factor++ {
		var err error
		if equality {
			result.equal[factor], err = s.literal(AtomEQ, candidate, factor)
			if err != nil {
				return factorRelationLiterals{}, err
			}
			continue
		}
		result.great[factor], err = s.literal(AtomGT, candidate, factor)
		if err != nil {
			return factorRelationLiterals{}, err
		}
		result.less[factor], err = s.literal(AtomLT, candidate, factor)
		if err != nil {
			return factorRelationLiterals{}, err
		}
	}
	return result, nil
}
