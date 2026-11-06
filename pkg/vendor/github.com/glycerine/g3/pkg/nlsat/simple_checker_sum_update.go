package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1006-1037.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (d *simplePowerDomain) applySumTerm(am *polynomial.AlgebraicManager, t *simpleSumTerm, c *polynomial.Algebraic, sign simpleSign) (changed, consistent bool, err error) {
	form := simplePowerForm{variable: t.variable, degree: t.degree, sign: sign}
	defer func() { err = errors.Join(err, form.close(am)) }()
	if err = am.Copy(&form.a, &t.coefficient); err != nil {
		return false, false, err
	}
	if err = am.Copy(&form.c, c); err != nil {
		return false, false, err
	}
	neg, err := am.IsNeg(&form.a)
	if err != nil {
		return false, false, err
	}
	if neg {
		form.sign = form.sign.flipped()
	}
	if err = am.Div(&form.c, &form.a, &form.boundary); err != nil {
		return false, false, err
	}
	if err = am.Neg(&form.boundary); err != nil {
		return false, false, err
	}
	form.thresholdSign, err = am.Compare(&form.boundary, &polynomial.Algebraic{})
	if err != nil {
		return false, false, err
	}
	if form.degree&1 == 1 || form.thresholdSign >= 0 {
		if err = am.Root(&form.boundary, form.degree, &form.boundary); err != nil {
			return false, false, err
		}
	}
	return d.apply(am, &form)
}
