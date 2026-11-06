package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:280-299,809-812.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func polynomialConstantUnit(core *polynomial.MultivariatePolynomialCore, p polynomial.Polynomial) (bool, error) {
	constant, err := core.IsConstant(p)
	if err != nil || !constant {
		return false, err
	}
	info, err := core.Info(p)
	if err != nil || info.Size != 1 {
		return false, err
	}
	coefficient, err := core.Coefficient(p, 0)
	if err != nil {
		return false, err
	}
	integers, err := core.IntegerManager()
	if err != nil {
		return false, err
	}
	one, err := integers.IsOne(&coefficient)
	if err != nil || one {
		return one, err
	}
	return integers.IsMinusOne(&coefficient)
}

func (s *unconstrainedEqualitySimplifier) linearCoefficient(variable uint32, p polynomial.Polynomial) (coefficient, reduct *polynomial.PolynomialRef, matched bool, err error) {
	core, err := s.solver.config.Polynomial.ArithmeticCore()
	if err != nil {
		return nil, nil, false, err
	}
	coefficient, err = polynomial.NewPolynomialRef(core)
	if err != nil {
		return nil, nil, false, err
	}
	reduct, err = polynomial.NewPolynomialRef(core)
	if err != nil {
		return nil, nil, false, errors.Join(err, coefficient.Close())
	}
	var call polynomial.PolynomialCoefficientCall
	if err = core.CoeffWithReductInto(p, variable, 1, coefficient, reduct, &call); err != nil {
		return nil, nil, false, errors.Join(err, call.CleanupRetained(), reduct.Close(), coefficient.Close())
	}
	value, err := coefficient.Get()
	if err != nil {
		return nil, nil, false, errors.Join(err, reduct.Close(), coefficient.Close())
	}
	constant, err := core.IsConstant(value)
	if err != nil || !constant {
		return nil, nil, false, errors.Join(err, reduct.Close(), coefficient.Close())
	}
	if s.solver.clauses.vars.isInt[variable] {
		unit, cause := polynomialConstantUnit(core, value)
		if cause != nil || !unit {
			return nil, nil, false, errors.Join(cause, reduct.Close(), coefficient.Close())
		}
	}
	return coefficient, reduct, true, nil
}
