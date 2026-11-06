package realclosure

// Source: src/math/realclosure/realclosure.h:310-320, concrete mpq
// instantiations of the scoped binary operators.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) addValueRational(value *realValue, scalar *numeral.Rational, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newRationalCopy(scalar)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.addAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) subtractValueRational(value *realValue, scalar *numeral.Rational, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newRationalCopy(scalar)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.subtractAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) multiplyValueRational(value *realValue, scalar *numeral.Rational, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newRationalCopy(scalar)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.multiplyAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) divideValueRational(value *realValue, scalar *numeral.Rational, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newRationalCopy(scalar)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.divideRankedRetainedValues(value, temporary, usePrem)
}
