package realclosure

// Source: src/math/realclosure/realclosure.cpp:4483-4516, 4867-4880.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) retainedValueRank(value *realValue) (int, error) {
	if value == nil || value.isRational {
		if value != nil && (value.store != s || value.deleted || value.refs == 0) {
			return 0, arena.ErrInvalidSlot
		}
		return -1, nil
	}
	if value.store != s || value.deleted || value.refs == 0 || value.root == nil || !value.root.isRegistered() {
		return 0, arena.ErrInvalidSlot
	}
	return value.root.index, nil
}

func (s *realValueStore) compareRetainedValueRanks(left, right *realValue) (int, error) {
	leftRank, err := s.retainedValueRank(left)
	if err != nil {
		return 0, err
	}
	rightRank, err := s.retainedValueRank(right)
	if err != nil {
		return 0, err
	}
	if left != nil && right != nil && !left.isRational && !right.isRational && left.root.owner != right.root.owner {
		return 0, arena.ErrInvalidSlot
	}
	if left != nil && right != nil && !left.isRational && !right.isRational && left.root.kind != right.root.kind {
		leftKind := sourceExtensionRank(left.root.kind)
		rightKind := sourceExtensionRank(right.root.kind)
		if leftKind < rightKind {
			return -1, nil
		}
		return 1, nil
	}
	if leftRank < rightRank {
		return -1, nil
	}
	if leftRank > rightRank {
		return 1, nil
	}
	return 0, nil
}

// sourceExtensionRank mirrors extension::kind in realclosure.cpp, whose
// ordering is transcendental, infinitesimal, then algebraic.
func sourceExtensionRank(kind realExtensionKind) uint8 {
	switch kind {
	case realExtensionTranscendental:
		return 0
	case realExtensionInfinitesimal:
		return 1
	default:
		return 2
	}
}
