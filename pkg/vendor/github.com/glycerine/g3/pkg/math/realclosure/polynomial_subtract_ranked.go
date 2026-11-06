package realclosure

// Source: src/math/realclosure/realclosure.cpp:2690-2751.

import "errors"

func (s *realValueStore) subtractRankedPolynomials(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
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
		result[i], err = s.subtractRankedRetainedValues(a, b, usePrem)
		if err != nil {
			return nil, err
		}
	}
	return trimAnyPolynomial(result), nil
}
