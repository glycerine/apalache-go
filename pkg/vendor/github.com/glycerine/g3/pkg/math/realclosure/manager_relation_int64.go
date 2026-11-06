package realclosure

// Source: src/math/realclosure/realclosure.h:296-305, concrete int
// instantiations of the scoped comparison operators.

import "errors"

func (s *realValueStore) compareValueInt64(value *realValue, scalar int64) (comparison int, err error) {
	temporary, err := s.newRational(scalar, 1)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.compareValues(value, temporary, nil)
}

func (s *realValueStore) equalValueInt64(value *realValue, scalar int64) (bool, error) {
	c, err := s.compareValueInt64(value, scalar)
	return c == 0, err
}
func (s *realValueStore) notEqualValueInt64(value *realValue, scalar int64) (bool, error) {
	equal, err := s.equalValueInt64(value, scalar)
	if err != nil {
		return false, err
	}
	return !equal, nil
}
func (s *realValueStore) lessValueInt64(value *realValue, scalar int64) (bool, error) {
	c, err := s.compareValueInt64(value, scalar)
	return c < 0, err
}
func (s *realValueStore) greaterValueInt64(value *realValue, scalar int64) (bool, error) {
	c, err := s.compareValueInt64(value, scalar)
	return c > 0, err
}
func (s *realValueStore) lessEqualValueInt64(value *realValue, scalar int64) (bool, error) {
	greater, err := s.greaterValueInt64(value, scalar)
	if err != nil {
		return false, err
	}
	return !greater, nil
}
func (s *realValueStore) greaterEqualValueInt64(value *realValue, scalar int64) (bool, error) {
	less, err := s.lessValueInt64(value, scalar)
	if err != nil {
		return false, err
	}
	return !less, nil
}
