package realclosure

// Source: src/math/realclosure/realclosure.cpp:4671-4745, select q's sign
// beneath the already unique sign-condition column for one published root.

import "github.com/glycerine/g3/pkg/arena"

func selectExtendedRootSign(root *algebraicRoot, cardinalities []int, qMatrix *signMatrix) (int32, error) {
	if root == nil || root.signs == nil || root.signIndex < 0 || root.signIndex >= root.signs.matrix.cols || qMatrix == nil || qMatrix.cols < 2 {
		return 0, arena.ErrInvalidSlot
	}
	start := root.signIndex * qMatrix.cols
	selected := -1
	for child := 0; child < qMatrix.cols; child++ {
		index := start + child
		if index >= len(cardinalities) || cardinalities[index] < 0 || cardinalities[index] > 1 {
			return 0, arena.ErrInvalidSlot
		}
		if cardinalities[index] == 1 {
			if selected != -1 {
				return 0, arena.ErrInvalidSlot
			}
			selected = child
		}
	}
	if selected == -1 {
		return 0, arena.ErrInvalidSlot
	}
	sign := qMatrix.at(1, selected).Int64()
	if sign < 0 {
		return -1, nil
	}
	if sign > 0 {
		return 1, nil
	}
	return 0, nil
}
