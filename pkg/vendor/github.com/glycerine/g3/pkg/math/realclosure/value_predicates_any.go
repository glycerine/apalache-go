package realclosure

// Source: src/math/realclosure/realclosure.cpp:6197-6239.

func (s *realValueStore) equalAnyDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	comparison, err := s.compareAnyDeterminedValues(left, right, usePrem)
	return comparison == 0, err
}

func (s *realValueStore) lessAnyDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	comparison, err := s.compareAnyDeterminedValues(left, right, usePrem)
	return comparison < 0, err
}

func (s *realValueStore) greaterAnyDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	comparison, err := s.compareAnyDeterminedValues(left, right, usePrem)
	return comparison > 0, err
}
