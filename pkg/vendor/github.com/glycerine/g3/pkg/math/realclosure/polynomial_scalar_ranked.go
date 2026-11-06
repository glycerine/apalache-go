package realclosure

// Source: src/math/realclosure/realclosure.cpp:2788-2808, 2869-2877.

import "errors"

func (s *realValueStore) divideRankedPolynomialByScalar(p []*realValue, scalar *realValue, usePrem bool) (result []*realValue, err error) {
	result = make([]*realValue, len(p))
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i, coefficient := range p {
		result[i], err = s.divideRankedRetainedValues(coefficient, scalar, usePrem)
		if err != nil {
			return nil, err
		}
	}
	return trimAnyPolynomial(result), nil
}

func (s *realValueStore) makeRankedPolynomialMonic(p []*realValue, usePrem bool) ([]*realValue, error) {
	if err := s.checkAnyPolynomial(p, true); err != nil {
		return nil, err
	}
	if len(p) == 0 {
		return nil, nil
	}
	return s.divideRankedPolynomialByScalar(p, p[len(p)-1], usePrem)
}
