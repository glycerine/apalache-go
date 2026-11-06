package nlsat

// Source: src/nlsat/nlsat_solver.cpp:429-451,3155-3169,3230-3241;
// src/nlsat/nlsat_solver.h:132,159-163,193-197.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// GetRealValues copies the current arithmetic assignment into out.
func (s *Solver) GetRealValues(out *Assignment) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return out.Copy(s.assignment)
}

// SetRealValues restores a previously copied arithmetic assignment.
func (s *Solver) SetRealValues(values *Assignment) error {
	if err := s.open(); err != nil {
		return err
	}
	if values == nil {
		return arena.ErrConfig
	}
	return s.assignment.Copy(values)
}

// GetBooleanValues replaces out with the current Boolean assignment.
func (s *Solver) GetBooleanValues(out *[]lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	*out = append((*out)[:0], s.atoms.bools.values...)
	return nil
}

// SetBooleanValues restores a Boolean assignment and pads missing solver
// variables with undef, matching the source resize after copying.
func (s *Solver) SetBooleanValues(values []lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	n := len(s.atoms.bools.atoms)
	if cap(s.atoms.bools.values) < n {
		return arena.ErrBounds
	}
	s.atoms.bools.values = s.atoms.bools.values[:n]
	for i := range s.atoms.bools.values {
		s.atoms.bools.values[i] = lbool.Undef
	}
	copy(s.atoms.bools.values, values)
	return nil
}

// LiteralValue evaluates a literal from the current Boolean/arithmetic model.
func (s *Solver) LiteralValue(literal sat.Literal) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.search.value(literal)
}

func (s *Solver) polynomialVariables(p polynomial.Polynomial, out []uint32) ([]uint32, error) {
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return nil, err
	}
	monomials, err := core.MonomialManager()
	if err != nil {
		return nil, err
	}
	info, err := core.Info(p)
	if err != nil {
		return nil, err
	}
	found := make(map[uint32]bool)
	for i := uint32(0); i < info.Size; i++ {
		monomial, cause := core.TermMonomial(p, i)
		if cause != nil {
			return nil, cause
		}
		monomialInfo, cause := monomials.Info(monomial)
		if cause != nil {
			return nil, cause
		}
		for j := uint32(0); j < monomialInfo.Size; j++ {
			power, cause := monomials.ReadPower(monomial, j)
			if cause != nil {
				return nil, cause
			}
			if !found[power.Var] {
				found[power.Var] = true
				out = append(out, power.Var)
			}
		}
	}
	return out, nil
}

// LiteralVariables returns the source variable enumeration for an arithmetic
// literal. Inequality factors deduplicate independently; root atoms append the
// distinguished root variable after the polynomial variables.
func (s *Solver) LiteralVariables(literal sat.Literal, out *[]uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	*out = (*out)[:0]
	atom, found, err := s.AtomForBooleanVariable(literal.Variable())
	if err != nil || !found {
		return err
	}
	switch atom := atom.(type) {
	case *InequalityAtom:
		for i := uint32(0); i < atom.Size(); i++ {
			factor, cause := atom.Polynomial(i)
			if cause != nil {
				return cause
			}
			*out, cause = s.polynomialVariables(factor, *out)
			if cause != nil {
				return cause
			}
		}
		return nil
	case *RootAtom:
		*out, err = s.polynomialVariables(atom.Polynomial(), *out)
		if err == nil {
			*out = append(*out, atom.Variable())
		}
		return err
	default:
		return arena.ErrConfig
	}
}

// ArithmeticLiterals replaces out with the positive literal for every live
// arithmetic atom in Boolean-variable order.
func (s *Solver) ArithmeticLiterals(out *[]sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	*out = (*out)[:0]
	for variable, atom := range s.atoms.bools.atoms {
		if atom != nil && !s.atoms.bools.dead[variable] {
			*out = append(*out, sat.MakeLiteral(sat.BoolVar(variable), false))
		}
	}
	return nil
}
