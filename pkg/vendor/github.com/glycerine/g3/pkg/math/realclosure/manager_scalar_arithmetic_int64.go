package realclosure

// Source: src/math/realclosure/realclosure.h:310-320, concrete int
// instantiations of the scoped binary operators.

import "errors"

func (s *realValueStore) addValueInt64(value *realValue, scalar int64, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newRational(scalar, 1)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.addAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) subtractValueInt64(value *realValue, scalar int64, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newRational(scalar, 1)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.subtractAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) multiplyValueInt64(value *realValue, scalar int64, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newRational(scalar, 1)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.multiplyAnyCanonicalValues(value, temporary, usePrem)
}

func (s *realValueStore) divideValueInt64(value *realValue, scalar int64, usePrem bool) (result *realValue, err error) {
	temporary, err := s.newRational(scalar, 1)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.divideRankedRetainedValues(value, temporary, usePrem)
}
