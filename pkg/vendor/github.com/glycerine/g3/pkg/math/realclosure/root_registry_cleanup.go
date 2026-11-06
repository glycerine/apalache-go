package realclosure

// Source: src/math/realclosure/realclosure.cpp:706-727.

import "github.com/glycerine/g3/pkg/arena"

func (o *algebraicRootOwner) cleanupTrailingRoots() error {
	if o == nil || o.values == nil || o.intervals == nil {
		return arena.ErrInvalidSlot
	}
	for len(o.roots) > 0 && o.roots[len(o.roots)-1] == nil {
		o.roots = o.roots[:len(o.roots)-1]
	}
	return nil
}
