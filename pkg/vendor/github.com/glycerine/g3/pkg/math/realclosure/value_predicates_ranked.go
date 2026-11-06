package realclosure

// Source: src/math/realclosure/realclosure.cpp:2449-2495, 6008-6045.

func (s *realValueStore) isZeroRankedValue(value *realValue, usePrem bool) (bool, error) {
	sign, err := s.determineRankedRetainedValueSign(value, usePrem)
	return sign == 0, err
}

func (s *realValueStore) isPositiveRankedValue(value *realValue, usePrem bool) (bool, error) {
	sign, err := s.determineRankedRetainedValueSign(value, usePrem)
	return sign > 0, err
}

func (s *realValueStore) isNegativeRankedValue(value *realValue, usePrem bool) (bool, error) {
	sign, err := s.determineRankedRetainedValueSign(value, usePrem)
	return sign < 0, err
}
