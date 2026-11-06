package realclosure

// Source: src/math/realclosure/realclosure.h:221-255, inline numeral
// relations derived from eq, lt, and gt.

func (s *realValueStore) notEqualAnyDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	equal, err := s.equalAnyDeterminedValues(left, right, usePrem)
	if err != nil {
		return false, err
	}
	return !equal, err
}

func (s *realValueStore) lessEqualAnyDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	greater, err := s.greaterAnyDeterminedValues(left, right, usePrem)
	if err != nil {
		return false, err
	}
	return !greater, err
}

func (s *realValueStore) greaterEqualAnyDeterminedValues(left, right *realValue, usePrem bool) (bool, error) {
	less, err := s.lessAnyDeterminedValues(left, right, usePrem)
	if err != nil {
		return false, err
	}
	return !less, err
}
