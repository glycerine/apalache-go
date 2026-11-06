package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1247-1290.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *simpleSumChecker) monomialInterval(term polynomial.Monomial, coefficient *numeral.Int, domains []simplePowerDomain) (result simpleInterval, err error) {
	am := c.solver.config.Algebraic
	var value polynomial.Algebraic
	if err = am.SetInteger(&value, coefficient); err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, am.Del(&value)) }()
	result = newSimpleInterval()
	if err = result.lower.set(am, &value, false); err != nil {
		return result, err
	}
	if err = result.upper.set(am, &value, false); err != nil {
		return result, err
	}
	mm, err := c.solver.atoms.ineq.core.MonomialManager()
	if err != nil {
		return result, err
	}
	info, err := mm.Info(term)
	if err != nil {
		return result, err
	}
	for i := uint32(0); i < info.Size; i++ {
		power, cause := mm.ReadPower(term, i)
		if cause != nil {
			return result, cause
		}
		if uint64(power.Var) >= uint64(len(domains)) {
			return result, nil
		}
		factor := &domains[power.Var].original
		if power.Degree&1 == 0 {
			factor = &domains[power.Var].magnitude
		}
		for j := uint32(0); j < power.Degree; j++ {
			next, cause := multiplySimpleInterval(am, &result, factor)
			if cause != nil {
				return result, cause
			}
			if cause = result.close(am); cause != nil {
				return next, cause
			}
			result = next
		}
	}
	return result, nil
}
