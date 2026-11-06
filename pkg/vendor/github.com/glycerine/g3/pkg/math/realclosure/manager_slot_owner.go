package realclosure

// Source: src/math/realclosure/realclosure.cpp:5676-5680, 6251-6255.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) validateNumeralSlot(slot **realValue) error {
	if s == nil || slot == nil {
		return arena.ErrInvalidSlot
	}
	value := *slot
	if value != nil && (value.store != s || value.deleted || value.refs == 0) {
		return arena.ErrInvalidSlot
	}
	return nil
}
