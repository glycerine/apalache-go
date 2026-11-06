package realclosure

// Source: src/math/realclosure/realclosure.cpp:5684-5720, exact subtraction
// and sign fallback after interval ordering is inconclusive.

import "errors"

func (s *realValueStore) compareDeterminedValues(left, right *realValue, usePrem bool) (comparison int, err error) {
	if left == right {
		return 0, nil
	}
	difference, err := s.subtractCanonicalValues(left, right, usePrem)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(difference)) }()
	sign, err := s.determineRetainedValueSign(difference, usePrem)
	return int(sign), err
}
