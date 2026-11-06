package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:490-610.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (s *Solver) extractFourierMotzkinBound(x uint32, clause *Clause) (_ *fourierMotzkinBound, positive, ok bool, err error) {
	if clause.Size() != 1 {
		return nil, false, false, nil
	}
	literal := clause.Literals()[0]
	raw, err := s.clauses.literalAtom(literal)
	if err != nil {
		return nil, false, false, err
	}
	atom, ok := raw.(*InequalityAtom)
	if !ok || atom.Size() != 1 {
		return nil, false, false, nil
	}
	odd, err := atom.IsOdd(0)
	if err != nil || !odd {
		return nil, false, false, err
	}
	if atom.kind == AtomEQ {
		return nil, false, false, nil
	}
	p, err := atom.Polynomial(0)
	if err != nil {
		return nil, false, false, err
	}
	core := s.atoms.ineq.core
	degree, err := core.Degree(p, x)
	if err != nil || degree != 1 {
		return nil, false, false, err
	}
	a, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return nil, false, false, err
	}
	b, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		_ = a.Close()
		return nil, false, false, err
	}
	bound := &fourierMotzkinBound{variable: x, a: a, b: b, clause: clause}
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, bound.close())
		}
	}()
	coefficient, err := core.CoeffWithReduct(p, x, 1, b)
	if err != nil {
		return nil, false, false, err
	}
	if err = a.Assign(coefficient); err != nil {
		return nil, false, false, err
	}
	invertible, err := s.fourierMotzkinCoefficientInvertible(x, coefficient)
	if err != nil || !invertible {
		return nil, false, false, err
	}
	positive, err = s.fourierMotzkinCoefficientPositive(coefficient)
	if err != nil {
		return nil, false, false, err
	}
	integer, err := s.IsInteger(x)
	if err != nil {
		return nil, false, false, err
	}
	if atom.kind == AtomLT {
		if literal.Sign() {
			bound.strict = false
		} else {
			if err = s.negateFourierMotzkinBound(bound); err != nil {
				return nil, false, false, err
			}
			positive = !positive
			bound.strict = !integer
			if integer {
				if err = s.shiftIntegerFourierMotzkinBound(bound, positive); err != nil {
					return nil, false, false, err
				}
			}
		}
	} else { // GT
		if literal.Sign() {
			if err = s.negateFourierMotzkinBound(bound); err != nil {
				return nil, false, false, err
			}
			positive = !positive
			bound.strict = false
		} else {
			bound.strict = !integer
			if integer {
				if err = s.shiftIntegerFourierMotzkinBound(bound, positive); err != nil {
					return nil, false, false, err
				}
			}
		}
	}
	keep = true
	return bound, positive, true, nil
}
