package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:585-596.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (s *Solver) extractFourierMotzkinEquality(x uint32, clause *Clause) (lower, upper *fourierMotzkinBound, ok bool, err error) {
	if clause.Size() != 1 {
		return nil, nil, false, nil
	}
	literal := clause.Literals()[0]
	if literal.Sign() {
		return nil, nil, false, nil
	}
	raw, err := s.clauses.literalAtom(literal)
	if err != nil {
		return nil, nil, false, err
	}
	atom, yes := raw.(*InequalityAtom)
	if !yes || atom.kind != AtomEQ || atom.Size() != 1 {
		return nil, nil, false, nil
	}
	odd, err := atom.IsOdd(0)
	if err != nil || !odd {
		return nil, nil, false, err
	}
	p, err := atom.Polynomial(0)
	if err != nil {
		return nil, nil, false, err
	}
	core := s.atoms.ineq.core
	degree, err := core.Degree(p, x)
	if err != nil || degree != 1 {
		return nil, nil, false, err
	}
	a, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return nil, nil, false, err
	}
	b, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		_ = a.Close()
		return nil, nil, false, err
	}
	lower = &fourierMotzkinBound{variable: x, a: a, b: b, clause: clause}
	ownedLower := lower
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, ownedLower.close())
		}
	}()
	coefficient, err := core.CoeffWithReduct(p, x, 1, b)
	if err != nil {
		return nil, nil, false, err
	}
	if err = a.Assign(coefficient); err != nil {
		return nil, nil, false, err
	}
	invertible, err := s.fourierMotzkinCoefficientInvertible(x, coefficient)
	if err != nil || !invertible {
		return nil, nil, false, err
	}
	upperA, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return nil, nil, false, err
	}
	upperB, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		_ = upperA.Close()
		return nil, nil, false, err
	}
	upper = &fourierMotzkinBound{variable: x, a: upperA, b: upperB, clause: clause}
	negative, err := core.Neg(coefficient)
	if err == nil {
		err = upperA.Assign(negative)
	}
	bv, cause := b.Get()
	if err == nil {
		err = cause
	}
	if err == nil {
		negative, err = core.Neg(bv)
	}
	if err == nil {
		err = upperB.Assign(negative)
	}
	if err != nil {
		return nil, nil, false, errors.Join(err, upper.close())
	}
	keep = true
	return lower, upper, true, nil
}
