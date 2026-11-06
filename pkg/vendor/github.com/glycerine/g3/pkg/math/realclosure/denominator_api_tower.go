package realclosure

// Source: src/math/realclosure/realclosure.cpp:3329-3357.

func (s *realValueStore) cleanTowerValueIfNeeded(value *realValue, usePrem bool) (numerator, denominator *realValue, err error) {
	clean, err := s.hasCleanTowerDenominator(value)
	if err != nil {
		return nil, nil, err
	}
	if clean {
		if err = s.retain(value); err != nil {
			return nil, nil, err
		}
		denominator, err = s.newRational(1, 1)
		return value, denominator, err
	}
	return s.cleanTowerValueDenominator(value, usePrem)
}
