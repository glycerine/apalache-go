package lp

// Source: src/util/lp/tail_matrix.h, actual double,double virtual operations.

// FloatTailMatrix is the actual polymorphic tail used by floating LU. Dense
// left applications omit the source's unused settings parameter. Sparse left
// applications receive live mutable settings because their drop tests use it.
type FloatTailMatrix interface {
	IsDense() bool
	ApplyLeft([]float64) error
	ApplyLeftToT(*IndexedFloatVector, *Settings) error
	ApplyRight([]float64) error
	ApplyRightIndexed(*IndexedFloatVector) error
}

func (p *FloatPermutation) ApplyLeftToT(w *IndexedFloatVector, _ *Settings) error {
	return p.ApplyLeftIndexed(w)
}

var _ FloatTailMatrix = (*FloatPermutation)(nil)
