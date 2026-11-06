package realclosure

// Source: src/math/realclosure/realclosure.cpp:4817-4821 and arithmetic
// result sites that replace an algebraic zero by the distinguished nil value.

func (s *realValueStore) canonicalizeRetainedResult(value *realValue, usePrem bool) (*realValue, error) {
	if value == nil || value.isRational {
		return value, nil
	}
	sign, err := s.determineRetainedValueSign(value, usePrem)
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
