package realclosure

// Source: src/math/realclosure/realclosure.cpp:6197-6204, 6217-6221,
// 6229-6233.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *realValueStore) equalValueRational(value *realValue, scalar *numeral.Rational) (bool, error) {
	comparison, err := s.compareValueRational(value, scalar)
	return comparison == 0, err
}

func (s *realValueStore) lessValueRational(value *realValue, scalar *numeral.Rational) (bool, error) {
	comparison, err := s.compareValueRational(value, scalar)
	return comparison < 0, err
}

func (s *realValueStore) greaterValueRational(value *realValue, scalar *numeral.Rational) (bool, error) {
	comparison, err := s.compareValueRational(value, scalar)
	return comparison > 0, err
}
