package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:159-273.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

type simpleInterval struct{ lower, upper simpleEndpoint }

func newSimpleInterval() simpleInterval {
	return simpleInterval{lower: simpleEndpoint{infinite: true, open: true, lower: true}, upper: simpleEndpoint{infinite: true, open: true}}
}
func (d *simpleInterval) close(am *polynomial.AlgebraicManager) error {
	return errors.Join(d.upper.close(am), d.lower.close(am))
}
func (d *simpleInterval) empty(am *polynomial.AlgebraicManager) (bool, error) {
	order, err := d.upper.compare(am, &d.lower)
	return order < 0, err
}

func (d *simpleInterval) intersect(am *polynomial.AlgebraicManager, sign simpleSign, bound *polynomial.Algebraic) (changed bool, consistent bool, err error) {
	candidate := newSimpleInterval()
	defer func() { err = errors.Join(err, candidate.close(am)) }()
	switch sign {
	case simpleEQ:
		if err = candidate.lower.set(am, bound, false); err == nil {
			err = candidate.upper.set(am, bound, false)
		}
	case simpleLT:
		err = candidate.upper.set(am, bound, true)
	case simpleLE:
		err = candidate.upper.set(am, bound, false)
	case simpleGT:
		err = candidate.lower.set(am, bound, true)
	case simpleGE:
		err = candidate.lower.set(am, bound, false)
	default:
		return false, true, nil
	}
	if err != nil {
		return false, false, err
	}
	order, cause := d.lower.compare(am, &candidate.lower)
	if cause != nil {
		return false, false, cause
	}
	if order < 0 {
		if candidate.lower.infinite {
			d.lower.infinite = true
		} else if cause = d.lower.set(am, &candidate.lower.value, candidate.lower.open); cause != nil {
			return false, false, cause
		}
		changed = true
	}
	order, cause = d.upper.compare(am, &candidate.upper)
	if cause != nil {
		return false, false, cause
	}
	if order > 0 {
		if candidate.upper.infinite {
			d.upper.infinite = true
		} else if cause = d.upper.set(am, &candidate.upper.value, candidate.upper.open); cause != nil {
			return false, false, cause
		}
		changed = true
	}
	empty, cause := d.empty(am)
	return changed, !empty, cause
}

func (d *simpleInterval) permits(am *polynomial.AlgebraicManager, sign simpleSign, bound *polynomial.Algebraic) (bool, error) {
	switch sign {
	case simpleNEQ:
		if d.lower.infinite || d.upper.infinite {
			return true, nil
		}
		order, err := am.Compare(&d.lower.value, &d.upper.value)
		if err != nil || order != 0 {
			return order != 0, err
		}
		order, err = am.Compare(&d.lower.value, bound)
		return order != 0, err
	case simpleEQ:
		if !d.lower.infinite {
			o, e := am.Compare(bound, &d.lower.value)
			if e != nil || o < 0 || (o == 0 && d.lower.open) {
				return false, e
			}
		}
		if !d.upper.infinite {
			o, e := am.Compare(bound, &d.upper.value)
			if e != nil || o > 0 || (o == 0 && d.upper.open) {
				return false, e
			}
		}
		return true, nil
	case simpleLT, simpleLE:
		if d.lower.infinite {
			return true, nil
		}
		o, e := am.Compare(&d.lower.value, bound)
		if e != nil {
			return false, e
		}
		return o < 0 || (o == 0 && sign == simpleLE && !d.lower.open), nil
	case simpleGT, simpleGE:
		if d.upper.infinite {
			return true, nil
		}
		o, e := am.Compare(&d.upper.value, bound)
		if e != nil {
			return false, e
		}
		return o > 0 || (o == 0 && sign == simpleGE && !d.upper.open), nil
	}
	return true, nil
}
