package realclosure

// Source: src/math/realclosure/realclosure.cpp:2879-2913, 2991-3030.

import "errors"

// signedRemainderAny computes -(left mod right) over the retained value field.
func (s *realValueStore) signedRemainderAny(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	if err = s.checkAnyPolynomial(left, true); err != nil {
		return nil, err
	}
	if err = s.checkAnyPolynomial(right, false); err != nil {
		return nil, err
	}
	if len(right) == 1 {
		return nil, nil
	}
	result, err = s.retainRationalPolynomial(left)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) ([]*realValue, error) {
		return nil, errors.Join(cause, s.releaseSturmCoefficients(result))
	}
	for len(result) >= len(right) {
		if err = s.checkpoint(); err != nil {
			return fail(err)
		}
		shift := len(result) - len(right)
		ratio, e := s.divideRetainedValues(result[len(result)-1], right[len(right)-1])
		if e != nil {
			return fail(e)
		}
		for i := 0; i+1 < len(right); i++ {
			product, x := s.multiplyAnyCanonicalValues(ratio, right[i], usePrem)
			if x != nil {
				_ = s.releaseSturmScratch(ratio)
				return fail(x)
			}
			index := shift + i
			updated, x := s.subtractAnyCanonicalValues(result[index], product, usePrem)
			x = errors.Join(x, s.releaseSturmScratch(product))
			if x != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(ratio)
				return fail(x)
			}
			if x = s.releaseSturmScratch(result[index]); x != nil {
				result[index] = nil
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(ratio)
				return fail(x)
			}
			result[index] = updated
		}
		if err = s.releaseSturmScratch(ratio); err != nil {
			return fail(err)
		}
		if err = s.releaseSturmScratch(result[len(result)-1]); err != nil {
			result[len(result)-1] = nil
			return fail(err)
		}
		result = trimAnyPolynomial(result[:len(result)-1])
	}
	negated, err := s.negateAnyPolynomial(result)
	if err != nil {
		return fail(err)
	}
	if err = s.releaseSturmCoefficients(result); err != nil {
		_ = s.releaseSturmCoefficients(negated)
		return nil, err
	}
	return negated, nil
}
