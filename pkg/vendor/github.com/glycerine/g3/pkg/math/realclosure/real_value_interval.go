package realclosure

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) approximation(v *realValue, root *algebraicRoot) (*BinaryInterval, error) {
	if !s.realOperand(v) || v == nil || root == nil || root.owner == nil || root.owner.values != s {
		return nil, arena.ErrInvalidSlot
	}
	if v.isRational {
		return s.cachedRationalApproxInterval(v, root.owner.intervals)
	}
	if v.root != root || v.functionInterval == nil {
		return nil, arena.ErrInvalidSlot
	}
	return v.functionInterval, nil
}

func (s *realValueStore) newApproximation(root *algebraicRoot) (*BinaryInterval, error) {
	if root == nil || root.owner == nil || root.owner.values != s {
		return nil, arena.ErrInvalidSlot
	}
	return NewBinaryInterval(root.owner.intervals)
}
