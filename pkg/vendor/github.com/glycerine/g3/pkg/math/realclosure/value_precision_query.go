package realclosure

// Source: src/math/realclosure/realclosure.cpp:605-608 and 4313-4344.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *realValueStore) retainedIntervalPrecise(value *realValue, bounds *numeral.BinaryRationalManager, precision uint32) (bool, error) {
	if value == nil {
		return true, nil
	}
	interval, err := s.valueInterval(value, bounds)
	if err != nil {
		return false, err
	}
	return binaryIntervalPrecise(interval, precision)
}
