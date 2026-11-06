package realclosure

// Source: src/math/realclosure/realclosure.cpp:2788-2808 and 2869-2877.

import "errors"

func (s *realValueStore) divideAnyPolynomialByScalar(p []*realValue, scalar *realValue, usePrem bool) (result []*realValue, err error) {
	result = make([]*realValue, len(p))
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i, coefficient := range p {
		result[i], err = s.divideRetainedValues(coefficient, scalar)
		if err != nil {
			return nil, err
		}
		result[i], err = s.canonicalizeAnyRetainedResult(result[i], usePrem)
		if err != nil {
			return nil, err
		}
	}
	return trimAnyPolynomial(result), nil
}

func (s *realValueStore) negateAnyPolynomial(p []*realValue) (result []*realValue, err error) {
	result = make([]*realValue, len(p))
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i, coefficient := range p {
		result[i], err = s.negateRetainedValue(coefficient)
		if err != nil {
			return nil, err
		}
	}
	return trimAnyPolynomial(result), nil
}
