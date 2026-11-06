package realclosure

// Source: src/math/realclosure/realclosure.h:221-255, concrete mpq
// inline relations.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *realValueStore) notEqualValueRational(value *realValue, scalar *numeral.Rational) (bool, error) {
	equal, err := s.equalValueRational(value, scalar)
	if err != nil {
		return false, err
	}
	return !equal, err
}

func (s *realValueStore) lessEqualValueRational(value *realValue, scalar *numeral.Rational) (bool, error) {
	greater, err := s.greaterValueRational(value, scalar)
	if err != nil {
		return false, err
	}
	return !greater, err
}

func (s *realValueStore) greaterEqualValueRational(value *realValue, scalar *numeral.Rational) (bool, error) {
	less, err := s.lessValueRational(value, scalar)
	if err != nil {
		return false, err
	}
	return !less, err
}
