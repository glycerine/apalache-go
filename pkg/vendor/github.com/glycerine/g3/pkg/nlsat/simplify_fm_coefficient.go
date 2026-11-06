package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:490-503.

import "github.com/glycerine/g3/pkg/math/polynomial"

func (s *Solver) fourierMotzkinCoefficientInvertible(x uint32, p polynomial.Polynomial) (bool, error) {
	core := s.atoms.ineq.core
	constant, err := core.IsConstant(p)
	if err != nil || !constant {
		return false, err
	}
	integer, err := s.IsInteger(x)
	if err != nil || !integer {
		return !integer, err
	}
	coefficient, err := core.Coefficient(p, 0)
	if err != nil {
		return false, err
	}
	z, err := core.IntegerManager()
	if err != nil {
		return false, err
	}
	one, err := z.IsOne(&coefficient)
	if err != nil {
		return false, err
	}
	minusOne, err := z.IsMinusOne(&coefficient)
	return one || minusOne, err
}

func (s *Solver) fourierMotzkinCoefficientPositive(p polynomial.Polynomial) (bool, error) {
	core := s.atoms.ineq.core
	coefficient, err := core.Coefficient(p, 0)
	if err != nil {
		return false, err
	}
	z, err := core.IntegerManager()
	if err != nil {
		return false, err
	}
	return z.IsPos(&coefficient)
}
