package realclosure

// Source: src/math/realclosure/realclosure.cpp:3360-3364.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) extensionIndex(value *realValue) (int, error) {
	if value == nil || value.isRational {
		return -1, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 || value.root == nil {
		return 0, arena.ErrInvalidSlot
	}
	return value.root.registryIndex()
}
