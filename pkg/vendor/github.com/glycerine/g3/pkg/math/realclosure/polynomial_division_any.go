package realclosure

// Source: src/math/realclosure/realclosure.cpp:2811-2867.

import (
	"errors"
)

func (s *realValueStore) divideAnyPolynomials(left, right []*realValue, usePrem bool) (quotient, remainder []*realValue, err error) {
	if err = s.checkAnyPolynomial(left, true); err != nil {
		return nil, nil, err
	}
	if err = s.checkAnyPolynomial(right, false); err != nil {
		return nil, nil, err
	}
	remainder, err = s.retainRationalPolynomial(left)
	if err != nil {
		return nil, nil, err
	}
	if len(left) >= len(right) {
		quotient = make([]*realValue, len(left)-len(right)+1)
	}
	fail := func(body error) ([]*realValue, []*realValue, error) {
		return nil, nil, errors.Join(body, s.releaseSturmCoefficients(quotient), s.releaseSturmCoefficients(remainder))
	}
	for len(remainder) >= len(right) {
		if err = s.checkpoint(); err != nil {
			return fail(err)
		}
		shift := len(remainder) - len(right)
		ratio, e := s.divideRetainedValues(remainder[len(remainder)-1], right[len(right)-1])
		if e != nil {
			return fail(e)
		}
		updatedQ, e := s.addAnyCanonicalValues(quotient[shift], ratio, usePrem)
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
			product, x := s.multiplyAnyCanonicalValues(ratio, right[i], usePrem)
			if x != nil {
				_ = s.releaseSturmScratch(ratio)
				return fail(x)
			}
			index := shift + i
			updated, x := s.subtractAnyCanonicalValues(remainder[index], product, usePrem)
			x = errors.Join(x, s.releaseSturmScratch(product))
			if x != nil {
				_ = s.releaseSturmScratch(ratio)
				return fail(x)
			}
			if x = s.releaseSturmScratch(remainder[index]); x != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(ratio)
				return fail(x)
			}
			remainder[index] = updated
		}
		if e = s.releaseSturmScratch(ratio); e != nil {
			return fail(e)
		}
		if e = s.releaseSturmScratch(remainder[len(remainder)-1]); e != nil {
			return fail(e)
		}
		remainder = trimAnyPolynomial(remainder[:len(remainder)-1])
	}
	quotient = trimAnyPolynomial(quotient)
	return quotient, remainder, nil
}
