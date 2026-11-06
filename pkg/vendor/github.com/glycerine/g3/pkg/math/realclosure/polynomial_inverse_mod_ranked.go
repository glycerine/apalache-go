package realclosure

// Source: src/math/realclosure/realclosure.cpp:5330-5403.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) invertRankedPolynomialModulo(q, modulus []*realValue, usePrem bool) (inverse []*realValue, coprime bool, err error) {
	if err = s.checkAnyPolynomial(q, false); err != nil {
		return nil, false, err
	}
	if err = s.checkAnyPolynomial(modulus, false); err != nil || len(q) >= len(modulus) {
		if err == nil {
			err = arena.ErrInvalidSlot
		}
		return nil, false, err
	}
	oldR, err := s.retainRationalPolynomial(modulus)
	if err != nil {
		return nil, false, err
	}
	r, err := s.retainRationalPolynomial(q)
	if err != nil {
		return nil, false, errors.Join(err, s.releaseSturmCoefficients(oldR))
	}
	oldT := []*realValue(nil)
	one, err := s.newRational(1, 1)
	if err != nil {
		return nil, false, errors.Join(err, s.releaseSturmCoefficients(oldR), s.releaseSturmCoefficients(r))
	}
	t := []*realValue{one}
	defer func() {
		err = errors.Join(err, s.releaseSturmCoefficients(oldR), s.releaseSturmCoefficients(r), s.releaseSturmCoefficients(oldT), s.releaseSturmCoefficients(t))
	}()
	for len(r) > 0 {
		quotient, remainder, e := s.divideRankedPolynomials(oldR, r, usePrem)
		if e != nil {
			return nil, false, e
		}
		product, e := s.multiplyRankedPolynomials(quotient, t, usePrem)
		if e != nil {
			_ = s.releaseSturmCoefficients(quotient)
			_ = s.releaseSturmCoefficients(remainder)
			return nil, false, e
		}
		nextT, e := s.subtractRankedPolynomials(oldT, product, usePrem)
		e = errors.Join(e, s.releaseSturmCoefficients(quotient), s.releaseSturmCoefficients(product))
		if e != nil {
			_ = s.releaseSturmCoefficients(remainder)
			_ = s.releaseSturmCoefficients(nextT)
			return nil, false, e
		}
		if e = errors.Join(s.releaseSturmCoefficients(oldR), s.releaseSturmCoefficients(oldT)); e != nil {
			_ = s.releaseSturmCoefficients(remainder)
			_ = s.releaseSturmCoefficients(nextT)
			oldR, oldT = nil, nil
			return nil, false, e
		}
		oldR, r = r, remainder
		oldT, t = t, nextT
	}
	if len(oldR) != 1 || oldR[0] == nil {
		return nil, false, nil
	}
	inverse, err = s.divideRankedPolynomialByScalar(oldT, oldR[0], usePrem)
	if err != nil {
		return nil, false, err
	}
	if len(inverse) >= len(modulus) {
		quotient, reduced, e := s.divideRankedPolynomials(inverse, modulus, usePrem)
		e = errors.Join(e, s.releaseSturmCoefficients(quotient), s.releaseSturmCoefficients(inverse))
		if e != nil {
			_ = s.releaseSturmCoefficients(reduced)
			return nil, false, e
		}
		inverse = reduced
	}
	return inverse, true, nil
}
