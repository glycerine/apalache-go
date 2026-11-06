package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:162-239.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *factorSimplifier) replacement(candidate factorSimplificationCandidate) ([][]sat.Literal, error) {
	switch candidate.kind {
	case AtomEQ:
		return s.equalityReplacement(candidate)
	case AtomLT:
		return s.lessReplacement(candidate)
	case AtomGT:
		return s.greaterReplacement(candidate)
	default:
		return nil, arena.ErrConfig
	}
}
