package realclosure

// Source: src/math/realclosure/realclosure.cpp:5137-5159.

func (s *realValueStore) multiplyCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	result, err := s.multiplyRetainedValues(left, right)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeRetainedResult(result, usePrem)
}
