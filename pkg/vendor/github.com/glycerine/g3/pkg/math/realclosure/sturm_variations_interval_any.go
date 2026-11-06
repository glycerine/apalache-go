package realclosure

// Source: src/math/realclosure/realclosure.cpp:3930-3997.

func (s *realValueStore) signVariationsAtBinaryEndpointAny(rows []realPolynomial, interval *BinaryInterval, lower, usePrem bool) (int, error) {
	if err := interval.open(); err != nil {
		return 0, err
	}
	if lower && interval.lowerInf {
		return s.signVariationsAtInfinity(rows, false)
	}
	if !lower && interval.upperInf {
		return s.signVariationsAtInfinity(rows, true)
	}
	bound := &interval.upper
	if lower {
		bound = &interval.lower
	}
	previous, variations := int32(0), 0
	for _, row := range rows {
		sign, err := s.signAnyPolynomialAtBinaryBound(row.coefficients, interval.numbers, bound, usePrem)
		if err != nil {
			return 0, err
		}
		if sign == 0 {
			continue
		}
		if previous != 0 && sign != previous {
			variations++
		}
		previous = sign
	}
	return variations, nil
}

func (s *realValueStore) tarskiFromBinaryIntervalSequenceAny(rows []realPolynomial, interval *BinaryInterval, usePrem bool) (int, error) {
	lower, err := s.signVariationsAtBinaryEndpointAny(rows, interval, true, usePrem)
	if err != nil {
		return 0, err
	}
	upper, err := s.signVariationsAtBinaryEndpointAny(rows, interval, false, usePrem)
	if err != nil {
		return 0, err
	}
	return lower - upper, nil
}
