package realclosure

// Source: src/math/realclosure/realclosure.cpp:2879-2913, 2991-3030.

import "errors"

func (s *realValueStore) signedRemainderRanked(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
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
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for len(result) >= len(right) {
		if err = s.checkpoint(); err != nil {
			return nil, err
		}
		shift := len(result) - len(right)
		ratio, e := s.divideRankedRetainedValues(result[len(result)-1], right[len(right)-1], usePrem)
		if e != nil {
			return nil, e
		}
		for i := 0; i+1 < len(right); i++ {
			product, x := s.multiplyRankedRetainedValues(ratio, right[i], usePrem)
			if x != nil {
				_ = s.releaseSturmScratch(ratio)
				return nil, x
			}
			index := shift + i
			updated, x := s.subtractRankedRetainedValues(result[index], product, usePrem)
			x = errors.Join(x, s.releaseSturmScratch(product))
			if x != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(ratio)
				return nil, x
			}
			if x = s.releaseSturmScratch(result[index]); x != nil {
				result[index] = nil
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(ratio)
				return nil, x
			}
			result[index] = updated
		}
		if err = s.releaseSturmScratch(ratio); err != nil {
			return nil, err
		}
		if err = s.releaseSturmScratch(result[len(result)-1]); err != nil {
			result[len(result)-1] = nil
			return nil, err
		}
		result = trimAnyPolynomial(result[:len(result)-1])
	}
	negated := make([]*realValue, len(result))
	for i, coefficient := range result {
		negated[i], err = s.negateRankedRetainedValue(coefficient, usePrem)
		if err != nil {
			_ = s.releaseSturmCoefficients(negated)
			return nil, err
		}
	}
	if err = s.releaseSturmCoefficients(result); err != nil {
		result = nil
		_ = s.releaseSturmCoefficients(negated)
		return nil, err
	}
	result = trimAnyPolynomial(negated)
	return result, nil
}
