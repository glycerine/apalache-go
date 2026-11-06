package nlsat

// Source: src/nlsat/nlsat_solver.cpp:272-298,3190-3196;
// src/nlsat/nlsat_solver.h:98-101.

import "github.com/glycerine/g3/pkg/sat"

func (s *Solver) RetainBooleanVariable(variable sat.BoolVar) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.atoms.incRef(variable)
}

func (s *Solver) ReleaseBooleanVariable(variable sat.BoolVar) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.atoms.decRef(variable)
}

func (s *Solver) RetainLiteral(literal sat.Literal) error {
	return s.RetainBooleanVariable(literal.Variable())
}

func (s *Solver) ReleaseLiteral(literal sat.Literal) error {
	return s.ReleaseBooleanVariable(literal.Variable())
}
