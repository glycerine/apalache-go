package realclosure

// Source: src/math/realclosure/realclosure.cpp:4609-4670.

import "github.com/glycerine/g3/pkg/arena"

func solveCompletedSignExtension(existing *signDet, counts multiRootSignCounts, expanded []int) (signs signMatrix, cardinalities []int, qMatrix signMatrix, err error) {
	if existing == nil || existing.deleted || existing.matrix.rows == 0 || len(expanded) == 0 {
		return signMatrix{}, nil, signMatrix{}, arena.ErrInvalidSlot
	}
	var useful bool
	qMatrix, useful = signDetMatrix(counts.equal, counts.positive, counts.negative)
	if !useful {
		return signMatrix{}, nil, signMatrix{}, arena.ErrInvalidSlot
	}
	signs.tensorProduct(&existing.matrix, &qMatrix)
	if len(expanded) != signs.rows {
		return signMatrix{}, nil, signMatrix{}, arena.ErrInvalidSlot
	}
	var solvable bool
	cardinalities, solvable = signs.solveInt(expanded)
	if !solvable || len(cardinalities) != signs.cols {
		return signMatrix{}, nil, signMatrix{}, arena.ErrInvalidSlot
	}
	return signs, cardinalities, qMatrix, nil
}
