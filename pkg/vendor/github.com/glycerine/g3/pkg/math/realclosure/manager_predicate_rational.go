package realclosure

// Source: src/math/realclosure/realclosure.cpp:2520-2525, 6278-6280.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) queryValueIsRational(value *realValue) (bool, error) {
	if value == nil {
		return true, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 {
		return false, arena.ErrInvalidSlot
	}
	return value.isRational, nil
}
