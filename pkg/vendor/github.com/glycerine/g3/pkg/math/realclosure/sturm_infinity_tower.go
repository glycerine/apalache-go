package realclosure

// Source: src/math/realclosure/realclosure.cpp:3930-3997.

import "errors"

func (s *realValueStore) signVariationsAtInfinityTower(rows []realPolynomial, plus, usePrem bool) (int, error) {
	previous, variations := int32(0), 0
	for _, row := range rows {
		sign, err := s.determineTowerRetainedValueSign(row.coefficients[len(row.coefficients)-1], usePrem)
		if err != nil {
			return 0, err
		}
		if !plus && (len(row.coefficients)-1)%2 != 0 {
			sign = -sign
		}
		if sign != 0 {
			if previous != 0 && sign != previous {
				variations++
			}
			previous = sign
		}
	}
	return variations, nil
}

func (s *realValueStore) rootCountHalfLineTower(p []*realValue, positive, usePrem bool) (int, error) {
	derivative, err := s.derivativeTowerPolynomial(p, usePrem)
	if err != nil {
		return 0, err
	}
	rows, err := s.sturmSequenceRanked(p, derivative, usePrem)
	releaseErr := s.releaseSturmCoefficients(derivative)
	if err != nil {
		return 0, errors.Join(err, releaseErr)
	}
	if releaseErr != nil {
		_ = s.closePolynomialSequence(rows)
		return 0, releaseErr
	}
	zeroVariations := 0
	previous := int32(0)
	for _, row := range rows {
		sign, e := s.determineTowerRetainedValueSign(row.coefficients[0], usePrem)
		if e != nil {
			_ = s.closePolynomialSequence(rows)
			return 0, e
		}
		if sign != 0 {
			if previous != 0 && sign != previous {
				zeroVariations++
			}
			previous = sign
		}
	}
	infinity, err := s.signVariationsAtInfinityTower(rows, positive, usePrem)
	closeErr := s.closePolynomialSequence(rows)
	if err != nil {
		return 0, err
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if positive {
		return zeroVariations - infinity, nil
	}
	return infinity - zeroVariations, nil
}
