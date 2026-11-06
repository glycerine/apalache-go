package nlsat

// Source: src/nlsat/nlsat_solver.h:40-47; nlsat_simplify.cpp:508-610.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

type fourierMotzkinBound struct {
	variable uint32
	a, b     *polynomial.PolynomialRef
	strict   bool
	clause   *Clause
}

func (b *fourierMotzkinBound) close() error {
	if b == nil {
		return nil
	}
	return errors.Join(b.b.Close(), b.a.Close())
}

func (b *fourierMotzkinBound) values() (polynomial.Polynomial, polynomial.Polynomial, error) {
	a, err := b.a.Get()
	if err != nil {
		return polynomial.Polynomial{}, polynomial.Polynomial{}, err
	}
	bv, err := b.b.Get()
	return a, bv, err
}
