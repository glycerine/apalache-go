package realclosure

// Source: src/math/realclosure/realclosure.cpp:5684-5720.

import "errors"

func (s *realValueStore) compareRankedRetainedValues(left, right *realValue, usePrem bool) (comparison int, err error) {
	if left == right {
		return 0, nil
	}
	difference, err := s.subtractRankedRetainedValues(left, right, usePrem)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(difference)) }()
	sign, err := s.determineRankedRetainedValueSign(difference, usePrem)
	return int(sign), err
}
