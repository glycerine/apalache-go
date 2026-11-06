package realclosure

// Source: src/math/realclosure/realclosure.cpp:4923-5159.

func (s *realValueStore) addAnyCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	result, err := s.addRetainedValues(left, right)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeAnyRetainedResult(result, usePrem)
}

func (s *realValueStore) subtractAnyCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	result, err := s.subtractRetainedValues(left, right)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeAnyRetainedResult(result, usePrem)
}

func (s *realValueStore) multiplyAnyCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	result, err := s.multiplyRetainedValues(left, right)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeAnyRetainedResult(result, usePrem)
}
