package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:542-584.

import "github.com/glycerine/g3/pkg/math/polynomial"

func (s *Solver) negateFourierMotzkinBound(bound *fourierMotzkinBound) error {
	core := s.atoms.ineq.core
	a, b, err := bound.values()
	if err != nil {
		return err
	}
	negative, err := core.Neg(a)
	if err != nil {
		return err
	}
	if err = bound.a.Assign(negative); err != nil {
		return err
	}
	negative, err = core.Neg(b)
	if err != nil {
		return err
	}
	return bound.b.Assign(negative)
}

func (s *Solver) shiftIntegerFourierMotzkinBound(bound *fourierMotzkinBound, positive bool) error {
	core := s.atoms.ineq.core
	a, b, err := bound.values()
	if err != nil {
		return err
	}
	var result polynomial.Polynomial
	if positive {
		result, err = core.Sub(b, a)
	} else {
		result, err = core.Add(b, a)
	}
	if err != nil {
		return err
	}
	return bound.b.Assign(result)
}
