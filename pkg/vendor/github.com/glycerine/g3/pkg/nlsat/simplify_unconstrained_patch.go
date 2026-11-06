package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:289-298; nlsat_solver.cpp:2970-3022.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (s *unconstrainedEqualitySimplifier) rememberLinearEquality(variable uint32, p polynomial.Polynomial) (bool, error) {
	coefficient, reduct, matched, err := s.linearCoefficient(variable, p)
	if err != nil || !matched {
		return matched, err
	}
	core, err := s.solver.config.Polynomial.ArithmeticCore()
	if err != nil {
		return false, errors.Join(err, reduct.Close(), coefficient.Close())
	}
	reductValue, err := reduct.Get()
	if err != nil {
		return false, errors.Join(err, reduct.Close(), coefficient.Close())
	}
	numerator, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return false, errors.Join(err, reduct.Close(), coefficient.Close())
	}
	negative, err := core.Neg(reductValue)
	if err == nil {
		err = numerator.Assign(negative)
	}
	if err != nil {
		return false, errors.Join(err, numerator.Close(), reduct.Close(), coefficient.Close())
	}
	if err = reduct.Close(); err != nil {
		return false, errors.Join(err, numerator.Close(), coefficient.Close())
	}
	s.solver.rememberInlinePatch(variable, numerator, coefficient)
	return true, nil
}
