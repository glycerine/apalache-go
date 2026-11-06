package lp

// Source: src/util/lp/tail_matrix.h, actual precise virtual operations.

import "github.com/glycerine/g3/pkg/util/numeral"

type RationalTailMatrix interface {
	IsDense() bool
	Close() error
	ApplyLeft([]numeral.Rational) error
	ApplyLeftToT(*IndexedRationalVector, *Settings) error
	ApplyRight([]numeral.Rational) error
	ApplyRightIndexed(*IndexedRationalVector) error
}
type PairTailMatrix interface {
	IsDense() bool
	Close() error
	ApplyLeft([]RationalPair) error
	ApplyLeftToT(*IndexedRationalVector, *Settings) error
	ApplyRight([]numeral.Rational) error
	ApplyRightIndexed(*IndexedRationalVector) error
}

func (p *RationalPermutation) ApplyLeftToT(w *IndexedRationalVector, _ *Settings) error {
	return p.ApplyLeftIndexed(w)
}
func (p *PairPermutation) ApplyLeftToT(w *IndexedRationalVector, _ *Settings) error {
	return p.ApplyLeftIndexed(w)
}

var _ RationalTailMatrix = (*RationalEtaMatrix)(nil)
var _ RationalTailMatrix = (*RationalPermutation)(nil)
var _ PairTailMatrix = (*PairEtaMatrix)(nil)
var _ PairTailMatrix = (*PairPermutation)(nil)
