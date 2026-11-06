package realclosure

// Source: src/math/realclosure/realclosure.cpp:5057-5085.

import "errors"

func (s *realValueStore) subtractRankedRetainedValues(left, right *realValue, usePrem bool) (result *realValue, err error) {
	negative, err := s.negateRankedRetainedValue(right, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(negative)) }()
	return s.addRankedRetainedValues(left, negative, usePrem)
}
