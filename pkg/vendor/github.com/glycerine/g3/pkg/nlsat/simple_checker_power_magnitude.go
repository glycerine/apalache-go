package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:291-399.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (d *simplePowerDomain) refreshMagnitude(am *polynomial.AlgebraicManager) (consistent bool, err error) {
	lo, hi := &d.original.lower, &d.original.upper
	var zero, negative polynomial.Algebraic
	defer func() { err = errors.Join(err, am.Del(&negative), am.Del(&zero)) }()
	if err = am.SetInteger(&zero, ptrInteger(0)); err != nil {
		return false, err
	}
	if lo.infinite && hi.infinite {
		return true, nil
	}
	if lo.infinite {
		order, cause := am.Compare(&hi.value, &zero)
		if cause != nil {
			return false, cause
		}
		if order > 0 {
			return true, nil
		}
		if err = am.Copy(&negative, &hi.value); err != nil {
			return false, err
		}
		if err = am.Neg(&negative); err != nil {
			return false, err
		}
		_, consistent, err = d.magnitude.intersect(am, map[bool]simpleSign{true: simpleGT, false: simpleGE}[hi.open], &negative)
		return consistent, err
	}
	if hi.infinite {
		order, cause := am.Compare(&lo.value, &zero)
		if cause != nil {
			return false, cause
		}
		if order < 0 {
			return true, nil
		}
		sign := simpleGE
		if lo.open {
			sign = simpleGT
		}
		_, consistent, err = d.magnitude.intersect(am, sign, &lo.value)
		return consistent, err
	}
	loOrder, cause := am.Compare(&lo.value, &zero)
	if cause != nil {
		return false, cause
	}
	hiOrder, cause := am.Compare(&hi.value, &zero)
	if cause != nil {
		return false, cause
	}
	if loOrder >= 0 {
		sign := simpleGE
		if lo.open {
			sign = simpleGT
		}
		if _, consistent, err = d.magnitude.intersect(am, sign, &lo.value); err != nil || !consistent {
			return consistent, err
		}
		sign = simpleLE
		if hi.open {
			sign = simpleLT
		}
		_, consistent, err = d.magnitude.intersect(am, sign, &hi.value)
		return consistent, err
	}
	if hiOrder <= 0 {
		if err = am.Copy(&negative, &hi.value); err != nil {
			return false, err
		}
		if err = am.Neg(&negative); err != nil {
			return false, err
		}
		sign := simpleGE
		if hi.open {
			sign = simpleGT
		}
		if _, consistent, err = d.magnitude.intersect(am, sign, &negative); err != nil || !consistent {
			return consistent, err
		}
		if err = am.Copy(&negative, &lo.value); err != nil {
			return false, err
		}
		if err = am.Neg(&negative); err != nil {
			return false, err
		}
		sign = simpleLE
		if lo.open {
			sign = simpleLT
		}
		_, consistent, err = d.magnitude.intersect(am, sign, &negative)
		return consistent, err
	}
	if _, consistent, err = d.magnitude.intersect(am, simpleGE, &zero); err != nil || !consistent {
		return consistent, err
	}
	if err = am.Copy(&negative, &lo.value); err != nil {
		return false, err
	}
	if err = am.Neg(&negative); err != nil {
		return false, err
	}
	order, cause := am.Compare(&negative, &hi.value)
	if cause != nil {
		return false, cause
	}
	upper := &hi.value
	open := hi.open
	if order > 0 {
		upper = &negative
		open = lo.open
	} else if order == 0 {
		open = lo.open || hi.open
	}
	sign := simpleLE
	if open {
		sign = simpleLT
	}
	_, consistent, err = d.magnitude.intersect(am, sign, upper)
	return consistent, err
}
