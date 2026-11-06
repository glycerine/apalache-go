package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:403-581.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (d *simplePowerDomain) apply(am *polynomial.AlgebraicManager, f *simplePowerForm) (changed, consistent bool, err error) {
	if f.degree&1 == 1 {
		changed, consistent, err = d.original.intersect(am, f.sign, &f.boundary)
		if err != nil || !consistent {
			return
		}
		var ok bool
		ok, err = d.refreshMagnitude(am)
		return changed, ok, err
	}
	if f.sign == simpleNEQ {
		zero, err0 := am.IsZero(&f.c)
		if err0 != nil {
			return false, false, err0
		}
		if !zero {
			return false, true, nil
		}
		changed, consistent, err = d.magnitude.intersect(am, simpleGT, &polynomial.Algebraic{})
		return
	}
	if f.thresholdSign < 0 {
		switch f.sign {
		case simpleEQ, simpleLT, simpleLE:
			return false, false, nil
		default:
			return false, true, nil
		}
	}
	if f.thresholdSign == 0 {
		switch f.sign {
		case simpleEQ, simpleLE:
			changed, consistent, err = d.magnitude.intersect(am, simpleEQ, &f.boundary)
			return
		case simpleLT:
			return false, false, nil
		case simpleGT:
			changed, consistent, err = d.magnitude.intersect(am, simpleGT, &f.boundary)
			return
		default:
			return false, true, nil
		}
	}
	switch f.sign {
	case simpleEQ:
		return d.magnitude.intersect(am, simpleEQ, &f.boundary)
	case simpleGT:
		return d.magnitude.intersect(am, simpleGT, &f.boundary)
	case simpleGE:
		return d.magnitude.intersect(am, simpleGE, &f.boundary)
	case simpleLT, simpleLE:
		sign := simpleLT
		if f.sign == simpleLE {
			sign = simpleLE
		}
		if changed, consistent, err = d.magnitude.intersect(am, sign, &f.boundary); err != nil || !consistent {
			return
		}
		var negative polynomial.Algebraic
		defer func() { err = errors.Join(err, am.Del(&negative)) }()
		if err = am.Copy(&negative, &f.boundary); err != nil {
			return false, false, err
		}
		if err = am.Neg(&negative); err != nil {
			return false, false, err
		}
		lower := simpleGT
		if f.sign == simpleLE {
			lower = simpleGE
		}
		c1, ok, er := d.original.intersect(am, lower, &negative)
		changed = changed || c1
		if er != nil || !ok {
			return changed, ok, er
		}
		c2, ok, er := d.original.intersect(am, sign, &f.boundary)
		return changed || c2, ok, er
	}
	return false, true, nil
}
