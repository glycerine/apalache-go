package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:729-753.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (s *Solver) matchFourierMotzkinEquality(lo, hi []*fourierMotzkinBound) (lower, upper *fourierMotzkinBound, found bool, err error) {
	core := s.atoms.ineq.core
	negativeA, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return nil, nil, false, err
	}
	negativeB, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		_ = negativeA.Close()
		return nil, nil, false, err
	}
	defer func() { err = errors.Join(err, negativeB.Close(), negativeA.Close()) }()
	for _, l := range lo {
		if l.strict {
			continue
		}
		la, lb, cause := l.values()
		if cause != nil {
			return nil, nil, false, cause
		}
		value, cause := core.Neg(la)
		if cause == nil {
			cause = negativeA.Assign(value)
		}
		if cause == nil {
			value, cause = core.Neg(lb)
		}
		if cause == nil {
			cause = negativeB.Assign(value)
		}
		if cause != nil {
			return nil, nil, false, cause
		}
		na, cause := negativeA.Get()
		if cause != nil {
			return nil, nil, false, cause
		}
		nb, cause := negativeB.Get()
		if cause != nil {
			return nil, nil, false, cause
		}
		for _, h := range hi {
			if h.strict {
				continue
			}
			ha, hb, cause := h.values()
			if cause != nil {
				return nil, nil, false, cause
			}
			equalA, cause := core.Equal(na, ha)
			if cause != nil {
				return nil, nil, false, cause
			}
			equalB, cause := core.Equal(nb, hb)
			if cause != nil {
				return nil, nil, false, cause
			}
			if equalA && equalB {
				return l, h, true, nil
			}
		}
	}
	return nil, nil, false, nil
}
