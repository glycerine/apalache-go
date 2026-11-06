package realclosure

// Source: src/math/realclosure/realclosure.cpp:4817-4821.

func (s *realValueStore) canonicalizeRankedRetainedResult(value *realValue, usePrem bool) (*realValue, error) {
	if value == nil || value.isRational {
		return value, nil
	}
	sign, err := s.determineRankedRetainedValueSign(value, usePrem)
	if err != nil {
		return value, err
	}
	if sign != 0 {
		return value, nil
	}
	if err := s.release(value); err != nil {
		return value, err
	}
	return nil, nil
}
