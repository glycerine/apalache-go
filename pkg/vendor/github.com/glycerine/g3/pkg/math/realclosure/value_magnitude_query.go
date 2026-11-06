package realclosure

// Source: src/math/realclosure/realclosure.cpp:559-603.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *realValueStore) retainedIntervalMagnitude(value *realValue, bounds *numeral.BinaryRationalManager) (int32, error) {
	if value == nil {
		return -1 << 31, nil
	}
	interval, err := s.valueInterval(value, bounds)
	if err != nil {
		return 0, err
	}
	return s.sturmIntervalMagnitude(interval)
}
