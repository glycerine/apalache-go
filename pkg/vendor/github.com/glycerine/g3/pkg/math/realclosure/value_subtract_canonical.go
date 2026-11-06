package realclosure

// Source: src/math/realclosure/realclosure.cpp:5057-5085.

func (s *realValueStore) subtractCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	result, err := s.subtractRetainedValues(left, right)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeRetainedResult(result, usePrem)
}
