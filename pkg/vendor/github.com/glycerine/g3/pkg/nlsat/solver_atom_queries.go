package nlsat

// Source: src/nlsat/nlsat_solver.cpp:300-387.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) BooleanVariableMaxVariable(variable sat.BoolVar) (uint32, error) {
	if err := s.open(); err != nil {
		return NullVariable, err
	}
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return NullVariable, arena.ErrBounds
	}
	header := arithmeticAtomHeader(s.atoms.bools.atoms[variable])
	if header == nil {
		return NullVariable, nil
	}
	return header.MaxVariable(), nil
}

func (s *Solver) LiteralMaxVariable(literal sat.Literal) (uint32, error) {
	return s.BooleanVariableMaxVariable(literal.Variable())
}

func (s *Solver) ArithmeticAtomDegree(atom ArithmeticAtom) (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	header := arithmeticAtomHeader(atom)
	if header == nil {
		return 0, arena.ErrConfig
	}
	variable := header.BooleanVariable()
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] || s.atoms.bools.atoms[variable] != atom {
		return 0, ErrAtomCacheOwner
	}
	return s.clauses.atomDegree(atom)
}

func (s *Solver) LiteralDegree(literal sat.Literal) (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	variable := literal.Variable()
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return 0, arena.ErrBounds
	}
	atom := s.atoms.bools.atoms[variable]
	if atom == nil {
		return 0, nil
	}
	return s.clauses.atomDegree(atom)
}
