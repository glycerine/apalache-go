package realclosure

// Source: src/math/realclosure/realclosure.h:310-320, concrete mpz
// instantiations, and realclosure.cpp:6120-6122, 6139-6143.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) addValueInteger(value *realValue, scalar *numeral.Int, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newIntegerValue(scalar)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.addAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) subtractValueInteger(value *realValue, scalar *numeral.Int, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newIntegerValue(scalar)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.subtractAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) multiplyValueInteger(value *realValue, scalar *numeral.Int, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newIntegerValue(scalar)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.multiplyAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) divideValueInteger(value *realValue, scalar *numeral.Int, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newIntegerValue(scalar)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.divideRankedRetainedValues(value, temporary, usePrem)
}
