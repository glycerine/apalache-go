package realclosure

// Source: src/math/realclosure/realclosure.cpp:2788-2877.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) checkRationalPolynomial(p []*realValue, allowEmpty bool) error {
	if s == nil || s.numbers == nil || (!allowEmpty && len(p) == 0) || len(p) > 0 && p[len(p)-1] == nil {
		return arena.ErrInvalidSlot
	}
	for _, coefficient := range p {
		if !s.rationalOperand(coefficient) {
			return arena.ErrInvalidSlot
		}
	}
	return nil
}

func (s *realValueStore) divideRationalPolynomial(left, right []*realValue) (quotient, remainder []*realValue, err error) {
	if err = s.checkRationalPolynomial(left, true); err != nil {
		return nil, nil, err
	}
	if err = s.checkRationalPolynomial(right, false); err != nil {
		return nil, nil, err
	}
	remainder = append([]*realValue(nil), left...)
	for i, coefficient := range remainder {
		if err = s.retain(coefficient); err != nil {
			_ = s.releaseSturmCoefficients(remainder[:i])
			return nil, nil, err
		}
	}
	if len(left) >= len(right) {
		quotient = make([]*realValue, len(left)-len(right)+1)
	}
	fail := func(cause error) ([]*realValue, []*realValue, error) {
		_ = s.releaseSturmCoefficients(quotient)
		_ = s.releaseSturmCoefficients(remainder)
		return nil, nil, cause
	}
	for len(remainder) >= len(right) {
		shift := len(remainder) - len(right)
		ratio, e := s.divideRationals(remainder[len(remainder)-1], right[len(right)-1])
		if e != nil {
			return fail(e)
		}
		updatedQ, e := s.addRationals(quotient[shift], ratio)
		if e != nil {
			_ = s.releaseSturmScratch(ratio)
			return fail(e)
		}
		if e = s.releaseSturmScratch(quotient[shift]); e != nil {
			_ = s.releaseSturmScratch(updatedQ)
			_ = s.releaseSturmScratch(ratio)
			return fail(e)
		}
		quotient[shift] = updatedQ
		for i := 0; i+1 < len(right); i++ {
			product, e := s.multiplyRationals(ratio, right[i])
			if e != nil {
				_ = s.releaseSturmScratch(ratio)
				return fail(e)
			}
			index := shift + i
			updated, e := s.subtractRationals(remainder[index], product)
			if e != nil {
				_ = s.releaseSturmScratch(product)
				_ = s.releaseSturmScratch(ratio)
				return fail(e)
			}
			if e = s.releaseSturmScratch(product); e != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(ratio)
				return fail(e)
			}
			if e = s.release(remainder[index]); e != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(ratio)
				return fail(e)
			}
			remainder[index] = updated
		}
		if e = s.releaseSturmScratch(ratio); e != nil {
			return fail(e)
		}
		if e = s.release(remainder[len(remainder)-1]); e != nil {
			return fail(e)
		}
		remainder = remainder[:len(remainder)-1]
		for len(remainder) > 0 && remainder[len(remainder)-1] == nil {
			remainder = remainder[:len(remainder)-1]
		}
	}
	for len(quotient) > 0 && quotient[len(quotient)-1] == nil {
		quotient = quotient[:len(quotient)-1]
	}
	return quotient, remainder, nil
}
