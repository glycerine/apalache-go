package realclosure

// Source: src/math/realclosure/realclosure.cpp:3675-3702.

import "errors"

func (s *realValueStore) rootSturmSequenceRanked(p []*realValue, usePrem bool) (rows []realPolynomial, err error) {
	derivative, err := s.derivativeAnyPolynomial(p, usePrem)
	if err != nil {
		return nil, err
	}
	rows, err = s.sturmSequenceRanked(p, derivative, usePrem)
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

func (s *realValueStore) tarskiSturmSequenceRanked(p, q []*realValue, usePrem bool) (rows []realPolynomial, err error) {
	derivative, err := s.derivativeAnyPolynomial(p, usePrem)
	if err != nil {
		return nil, err
	}
	second, err := s.multiplyRankedPolynomials(derivative, q, usePrem)
	releaseErr := s.releaseSturmCoefficients(derivative)
	if err != nil {
		return nil, errors.Join(err, releaseErr)
	}
	if releaseErr != nil {
		_ = s.releaseSturmCoefficients(second)
		return nil, releaseErr
	}
	rows, err = s.sturmSequenceRanked(p, second, usePrem)
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
