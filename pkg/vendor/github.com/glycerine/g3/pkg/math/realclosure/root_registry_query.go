package realclosure

// Source: concrete retained algebraic extension registry inspection.

import "github.com/glycerine/g3/pkg/arena"

func (o *algebraicRootOwner) liveAlgebraicRoots() ([]*algebraicRoot, error) {
	if o == nil || o.values == nil || o.intervals == nil {
		return nil, arena.ErrInvalidSlot
	}
	result := make([]*algebraicRoot, 0, len(o.roots))
	for index, root := range o.roots {
		if root == nil {
			continue
		}
		if root.deleted || root.owner != o || root.index != index {
			return nil, arena.ErrInvalidSlot
		}
		result = append(result, root)
	}
	return result, nil
}
