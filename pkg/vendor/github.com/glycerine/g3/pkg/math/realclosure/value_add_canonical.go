package realclosure

// Source: src/math/realclosure/realclosure.cpp:4923-4945.

func (s *realValueStore) addCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	result, err := s.addRetainedValues(left, right)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeRetainedResult(result, usePrem)
}
