package realclosure

// Source: src/math/realclosure/realclosure.cpp:3675-3687.

import "errors"

func (s *realValueStore) rootSturmSequenceAny(p []*realValue, usePrem bool) (rows []realPolynomial, err error) {
	derivative, err := s.derivativeAnyPolynomial(p, usePrem)
	if err != nil {
		return nil, err
	}
	if len(derivative) == 0 {
		return nil, errors.Join(s.releaseSturmCoefficients(derivative), err)
	}
	rows, err = s.sturmSequenceAny(p, derivative, usePrem)
	releaseErr := s.releaseSturmCoefficients(derivative)
	if err != nil {
		return nil, err
	}
	if releaseErr != nil {
		_ = s.closePolynomialSequence(rows)
		return nil, releaseErr
	}
	return rows, nil
}
