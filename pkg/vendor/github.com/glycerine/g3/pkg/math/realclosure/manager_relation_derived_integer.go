package realclosure

// Source: src/math/realclosure/realclosure.h:221-255, concrete mpz
// inline relations.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *realValueStore) notEqualValueInteger(value *realValue, scalar *numeral.Int) (bool, error) {
	equal, err := s.equalValueInteger(value, scalar)
	if err != nil {
		return false, err
	}
	return !equal, err
}

func (s *realValueStore) lessEqualValueInteger(value *realValue, scalar *numeral.Int) (bool, error) {
	greater, err := s.greaterValueInteger(value, scalar)
	if err != nil {
		return false, err
	}
	return !greater, err
}

func (s *realValueStore) greaterEqualValueInteger(value *realValue, scalar *numeral.Int) (bool, error) {
	less, err := s.lessValueInteger(value, scalar)
	if err != nil {
		return false, err
	}
	return !less, err
}
