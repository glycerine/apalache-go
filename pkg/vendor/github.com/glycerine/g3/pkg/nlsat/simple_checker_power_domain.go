package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:220-246.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

type simplePowerDomain struct{ original, magnitude simpleInterval }

func newSimplePowerDomain(am *polynomial.AlgebraicManager) (d simplePowerDomain, err error) {
	d.original = newSimpleInterval()
	d.magnitude = newSimpleInterval()
	var zero polynomial.Algebraic
	if err = am.SetInteger(&zero, ptrInteger(0)); err == nil {
		_, _, err = d.magnitude.intersect(am, simpleGE, &zero)
	}
	err = errors.Join(err, am.Del(&zero))
	return d, err
}
func (d *simplePowerDomain) close(am *polynomial.AlgebraicManager) error {
	return errors.Join(d.magnitude.close(am), d.original.close(am))
}
func cloneSimpleEndpoint(am *polynomial.AlgebraicManager, src *simpleEndpoint) (dst simpleEndpoint, err error) {
	dst.infinite, dst.open, dst.lower = src.infinite, src.open, src.lower
	if !src.infinite {
		err = am.Copy(&dst.value, &src.value)
	}
	return
}
func (d *simplePowerDomain) clone(am *polynomial.AlgebraicManager) (out simplePowerDomain, err error) {
	out.original.lower, err = cloneSimpleEndpoint(am, &d.original.lower)
	if err == nil {
		out.original.upper, err = cloneSimpleEndpoint(am, &d.original.upper)
	}
	if err == nil {
		out.magnitude.lower, err = cloneSimpleEndpoint(am, &d.magnitude.lower)
	}
	if err == nil {
		out.magnitude.upper, err = cloneSimpleEndpoint(am, &d.magnitude.upper)
	}
	if err != nil {
		err = errors.Join(err, out.close(am))
	}
	return
}
