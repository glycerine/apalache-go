package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1292-1348.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (c *simpleSumChecker) polynomialInterval(p polynomial.Polynomial, domains []simplePowerDomain) (result simpleInterval, err error) {
	core := c.solver.atoms.ineq.core
	info, err := core.Info(p)
	if err != nil {
		return result, err
	}
	if info.Size == 0 {
		result = newSimpleInterval()
		var zero polynomial.Algebraic
		if err = c.solver.config.Algebraic.SetInteger(&zero, ptrInteger(0)); err != nil {
			return result, err
		}
		defer func() { err = errors.Join(err, c.solver.config.Algebraic.Del(&zero)) }()
		if err = result.lower.set(c.solver.config.Algebraic, &zero, false); err != nil {
			return result, err
		}
		err = result.upper.set(c.solver.config.Algebraic, &zero, false)
		return result, err
	}
	for i := uint32(0); i < info.Size; i++ {
		term, cause := core.TermMonomial(p, i)
		if cause != nil {
			return result, cause
		}
		coefficient, cause := core.Coefficient(p, i)
		if cause != nil {
			return result, cause
		}
		current, cause := c.monomialInterval(term, &coefficient, domains)
		if cause != nil {
			return result, cause
		}
		if i == 0 {
			result = current
			continue
		}
		sum, cause := addSimpleInterval(c.solver.config.Algebraic, &result, &current)
		closeCause := current.close(c.solver.config.Algebraic)
		if cause == nil {
			cause = closeCause
		}
		if cause != nil {
			return result, cause
		}
		if cause = result.close(c.solver.config.Algebraic); cause != nil {
			return sum, cause
		}
		result = sum
	}
	return result, nil
}
