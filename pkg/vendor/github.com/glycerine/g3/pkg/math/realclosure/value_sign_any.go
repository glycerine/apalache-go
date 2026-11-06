package realclosure

// Source: src/math/realclosure/realclosure.cpp:4747-4821, all retained
// rational-coefficient algebraic extension variants.

func (s *realValueStore) determineAnyRetainedValueSign(value *realValue, usePrem bool) (int32, error) {
	if value == nil || value.isRational {
		return s.signValue(value)
	}
	contains, err := value.functionInterval.ContainsZero()
	if err != nil {
		return 0, err
	}
	if !contains {
		return s.signValue(value)
	}
	var sign int32
	if value.root.signs == nil {
		sign, err = s.exactSingleRootPolynomialSign(value, usePrem)
	} else {
		sign, err = s.exactMultiRootPolynomialSign(value, usePrem)
	}
	if err != nil || sign == 0 {
		return sign, err
	}
	if err = restrictBinaryIntervalToSign(value.functionInterval, sign); err != nil {
		return 0, err
	}
	return sign, nil
}
