package realclosure

// Source: src/math/realclosure/realclosure.cpp:5684-5720.

import "errors"

func (s *realValueStore) compareAnyDeterminedValues(left, right *realValue, usePrem bool) (comparison int, err error) {
	if left == right {
		return 0, nil
	}
	difference, err := s.subtractAnyCanonicalValues(left, right, usePrem)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(difference)) }()
	sign, err := s.determineAnyRetainedValueSign(difference, usePrem)
	return int(sign), err
}
