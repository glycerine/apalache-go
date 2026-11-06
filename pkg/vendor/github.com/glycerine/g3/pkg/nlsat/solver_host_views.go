package nlsat

// Source: src/nlsat/nlsat_solver.cpp:249-255,3107-3145;
// src/nlsat/nlsat_solver.h:119-176.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) NumBooleanVariables() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.atoms.bools.count, nil
}

func (s *Solver) NumVariables() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.clauses.vars.isInt)), nil
}

func (s *Solver) IsInteger(variable uint32) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if uint64(variable) >= uint64(len(s.clauses.vars.isInt)) {
		return false, arena.ErrBounds
	}
	return s.clauses.vars.isInt[variable], nil
}

func (s *Solver) True() (sat.Literal, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return TrueLiteral, nil
}

// AtomSnapshot copies the Boolean-variable-indexed atom vector. Nil entries
// represent frontend Boolean variables and retired slots.
func (s *Solver) AtomSnapshot(out *[]ArithmeticAtom) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	*out = append((*out)[:0], s.atoms.bools.atoms...)
	return nil
}

// VariableEqualitySnapshot copies the current per-variable asserted equality
// view maintained by the search trail.
func (s *Solver) VariableEqualitySnapshot(out *[]ArithmeticAtom) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	*out = append((*out)[:0], s.clauses.vars.equalities...)
	return nil
}

// Evaluator returns the solver-owned borrowed evaluator, matching get_evaluator.
func (s *Solver) Evaluator() (*Evaluator, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.evaluator, nil
}
