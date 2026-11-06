package realclosure

// Source: src/math/realclosure/realclosure.cpp:5330-5418, concrete rational
// polynomial extended-GCD arithmetic.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) retainRationalPolynomial(p []*realValue) ([]*realValue, error) {
	result := append([]*realValue(nil), p...)
	for i, coefficient := range result {
		if err := s.retain(coefficient); err != nil {
			_ = s.releaseSturmCoefficients(result[:i])
			return nil, err
		}
	}
	return result, nil
}

func (s *realValueStore) subtractRationalPolynomials(left, right []*realValue) (result []*realValue, err error) {
	size := len(left)
	if len(right) > size {
		size = len(right)
	}
	result = make([]*realValue, size)
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i := 0; i < size; i++ {
		var a, b *realValue
		if i < len(left) {
			a = left[i]
		}
		if i < len(right) {
			b = right[i]
		}
		result[i], err = s.subtractRationals(a, b)
		if err != nil {
			return nil, err
		}
	}
	for len(result) > 0 && result[len(result)-1] == nil {
		result = result[:len(result)-1]
	}
	return result, nil
}

func (s *realValueStore) divideRationalPolynomialByScalar(p []*realValue, scalar *realValue) (result []*realValue, err error) {
	if scalar == nil || !s.rationalOperand(scalar) {
		return nil, arena.ErrInvalidSlot
	}
	result = make([]*realValue, len(p))
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i, coefficient := range p {
		result[i], err = s.divideRationals(coefficient, scalar)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// invertRationalPolynomialModulo returns h with q*h = 1 modulo modulus.
func (s *realValueStore) invertRationalPolynomialModulo(q, modulus []*realValue) (inverse []*realValue, coprime bool, err error) {
	if err = s.checkRationalPolynomial(q, false); err != nil {
		return nil, false, err
	}
	if err = s.checkRationalPolynomial(modulus, false); err != nil || len(q) >= len(modulus) {
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
		quotient, remainder, e := s.divideRationalPolynomial(oldR, r)
		if e != nil {
			return nil, false, e
		}
		product, e := s.multiplyRationalPolynomials(quotient, t)
		if e != nil {
			_ = s.releaseSturmCoefficients(quotient)
			_ = s.releaseSturmCoefficients(remainder)
			return nil, false, e
		}
		nextT, e := s.subtractRationalPolynomials(oldT, product)
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
	inverse, err = s.divideRationalPolynomialByScalar(oldT, oldR[0])
	if err != nil {
		return nil, false, err
	}
	if len(inverse) >= len(modulus) {
		quotient, reduced, e := s.divideRationalPolynomial(inverse, modulus)
		e = errors.Join(e, s.releaseSturmCoefficients(quotient), s.releaseSturmCoefficients(inverse))
		if e != nil {
			_ = s.releaseSturmCoefficients(reduced)
			return nil, false, e
		}
		inverse = reduced
	}
	return inverse, true, nil
}
