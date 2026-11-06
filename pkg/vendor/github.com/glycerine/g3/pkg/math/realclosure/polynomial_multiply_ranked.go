package realclosure

// Source: src/math/realclosure/realclosure.cpp:2753-2791.

import "errors"

func (s *realValueStore) multiplyRankedPolynomials(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	if len(left) == 0 || len(right) == 0 {
		return nil, nil
	}
	result = make([]*realValue, len(left)+len(right)-1)
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	if len(left) < len(right) {
		left, right = right, left
	}
	for i, a := range left {
		if err = s.checkpoint(); err != nil {
			return nil, err
		}
		for j, b := range right {
			product, e := s.multiplyRankedRetainedValues(a, b, usePrem)
			if e != nil {
				return nil, e
			}
			updated, e := s.addRankedRetainedValues(result[i+j], product, usePrem)
			e = errors.Join(e, s.releaseSturmScratch(product))
			if e != nil {
				return nil, e
			}
			if e = s.releaseSturmScratch(result[i+j]); e != nil {
				_ = s.releaseSturmScratch(updated)
				return nil, e
			}
			result[i+j] = updated
		}
	}
	return trimAnyPolynomial(result), nil
}
