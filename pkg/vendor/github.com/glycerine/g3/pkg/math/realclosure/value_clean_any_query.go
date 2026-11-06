package realclosure

// Source: src/math/realclosure/realclosure.cpp:3090-3125.

func (s *realValueStore) hasCleanAnyDenominator(value *realValue) (bool, error) {
	return s.hasCleanDenominator(value)
}
