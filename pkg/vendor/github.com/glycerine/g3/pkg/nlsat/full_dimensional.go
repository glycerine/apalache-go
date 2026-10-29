package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2242-2288.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *clauseStore) isFullDimensionalLiteral(literal sat.Literal) (bool, error) {
	atom, err := s.literalAtom(literal)
	if err != nil {
		return false, err
	}
	if atom == nil {
		return true, nil
	}
	switch arithmeticAtomHeader(atom).kind {
	case AtomEQ, AtomRootEQ, AtomRootLE, AtomRootGE:
		return literal.Sign(), nil
	case AtomLT, AtomGT, AtomRootLT, AtomRootGT:
		return !literal.Sign(), nil
	default:
		return false, arena.ErrConfig
	}
}

func (s *clauseStore) isFullDimensional() (bool, error) {
	for _, clause := range s.originals {
		for _, literal := range clause.Literals() {
			full, err := s.isFullDimensionalLiteral(literal)
			if err != nil || !full {
				return full, err
			}
		}
	}
	return true, nil
}
