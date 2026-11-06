package realclosure

// Source: src/math/realclosure/realclosure.cpp:6206-6211, 6223-6227,
// 6235-6239.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *realValueStore) equalValueInteger(value *realValue, scalar *numeral.Int) (bool, error) {
	comparison, err := s.compareValueInteger(value, scalar)
	return comparison == 0, err
}

func (s *realValueStore) lessValueInteger(value *realValue, scalar *numeral.Int) (bool, error) {
	comparison, err := s.compareValueInteger(value, scalar)
	return comparison < 0, err
}

func (s *realValueStore) greaterValueInteger(value *realValue, scalar *numeral.Int) (bool, error) {
	comparison, err := s.compareValueInteger(value, scalar)
	return comparison > 0, err
}
