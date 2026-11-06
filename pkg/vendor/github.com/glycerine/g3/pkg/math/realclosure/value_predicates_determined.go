package realclosure

// Source: src/math/realclosure/realclosure.cpp:6008-6030 and 6197-6239.

func (s *realValueStore) equalDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	comparison, err := s.compareDeterminedValues(left, right, usePrem)
	return comparison == 0, err
}

func (s *realValueStore) lessDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	comparison, err := s.compareDeterminedValues(left, right, usePrem)
	return comparison < 0, err
}

func (s *realValueStore) greaterDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	comparison, err := s.compareDeterminedValues(left, right, usePrem)
	return comparison > 0, err
}
