package realclosure

// Source: src/math/realclosure/realclosure.cpp:3689-3702.

import "errors"

func (s *realValueStore) tarskiSturmSequenceAny(p, q []*realValue, usePrem bool) (rows []realPolynomial, err error) {
	derivative, err := s.derivativeAnyPolynomial(p, usePrem)
	if err != nil {
		return nil, err
	}
	second, err := s.multiplyAnyPolynomials(derivative, q, usePrem)
	releaseErr := s.releaseSturmCoefficients(derivative)
	if err != nil {
		return nil, errors.Join(err, releaseErr)
	}
	if releaseErr != nil {
		_ = s.releaseSturmCoefficients(second)
		return nil, releaseErr
	}
	if len(second) == 0 {
		return nil, s.releaseSturmCoefficients(second)
	}
	rows, err = s.sturmSequenceAny(p, second, usePrem)
	releaseErr = s.releaseSturmCoefficients(second)
	if err != nil {
		return nil, err
	}
	if releaseErr != nil {
		_ = s.closePolynomialSequence(rows)
		return nil, releaseErr
	}
	return rows, nil
}
