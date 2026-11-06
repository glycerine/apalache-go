package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1099-1154.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func simpleEndpointIsZero(am *polynomial.AlgebraicManager, e *simpleEndpoint) (bool, error) {
	if e.infinite || e.open {
		return false, nil
	}
	return am.IsZero(&e.value)
}
func simpleEndpointIsNegative(am *polynomial.AlgebraicManager, e *simpleEndpoint) (bool, error) {
	if e.infinite {
		return e.lower, nil
	}
	order, err := am.Compare(&e.value, &polynomial.Algebraic{})
	if err != nil {
		return false, err
	}
	if order < 0 {
		return true, nil
	}
	return order == 0 && e.open && !e.lower, nil
}
func multiplySimpleEndpoint(am *polynomial.AlgebraicManager, a, b *simpleEndpoint, c *simpleEndpoint) (err error) {
	az, err := simpleEndpointIsZero(am, a)
	if err != nil {
		return err
	}
	bz, err := simpleEndpointIsZero(am, b)
	if err != nil {
		return err
	}
	if az || bz {
		c.infinite = false
		c.open = false
		return am.SetInteger(&c.value, ptrInteger(0))
	}
	an, err := simpleEndpointIsNegative(am, a)
	if err != nil {
		return err
	}
	bn, err := simpleEndpointIsNegative(am, b)
	if err != nil {
		return err
	}
	if a.infinite || b.infinite {
		c.infinite = true
		c.open = true
		c.lower = an != bn
		return nil
	}
	c.infinite = false
	c.open = a.open || b.open
	var product polynomial.Algebraic
	defer func() { err = errors.Join(err, am.Del(&product)) }()
	if err = am.Mul(&a.value, &b.value, &product); err != nil {
		return err
	}
	return am.Copy(&c.value, &product)
}
func copySimpleEndpoint(am *polynomial.AlgebraicManager, dst, src *simpleEndpoint) error {
	dst.infinite, dst.open, dst.lower = src.infinite, src.open, src.lower
	if src.infinite {
		return am.Del(&dst.value)
	}
	return am.Copy(&dst.value, &src.value)
}
