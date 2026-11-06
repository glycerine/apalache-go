package bv

// Source: src/math/interval/mod_interval.h:21-230 and
// src/tactic/bv/bv_bounds_tactic.cpp:61-153.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/math/interval"
)

func bvBoundsMaximum(width uint32) *big.Int {
	if width == 0 {
		return new(big.Int)
	}
	return new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))
}

func (b bvBoundsSet) modularInterval() (*interval.ModularInterval, bool) {
	b = b.normalize()
	if b.width == 0 || len(b.ranges) == 0 || len(b.ranges) > 2 {
		return nil, false
	}
	maximum := bvBoundsMaximum(b.width)
	for _, r := range b.ranges {
		if r.lo == nil || r.hi == nil || r.lo.Sign() < 0 || r.lo.Cmp(r.hi) > 0 || r.hi.Cmp(maximum) > 0 {
			return nil, false
		}
	}
	lower, upper := b.ranges[0].lo, b.ranges[0].hi
	if len(b.ranges) == 2 {
		if b.ranges[0].lo.Sign() != 0 || b.ranges[1].hi.Cmp(maximum) != 0 {
			return nil, false
		}
		lower, upper = b.ranges[1].lo, b.ranges[0].hi
	}
	v, err := interval.NewModularInterval(lower, upper, b.width, b.tight)
	return v, err == nil
}

func bvBoundsFromModularInterval(v *interval.ModularInterval) bvBoundsSet {
	if v == nil {
		return bvBoundsSet{}
	}
	b := bvBoundsSet{width: v.Size(), tight: v.Tight()}
	lower, upper := v.Lower(), v.Upper()
	if v.IsWrapped() {
		b.ranges = []bvBoundsRange{
			{lo: big.NewInt(0), hi: upper},
			{lo: lower, hi: bvBoundsMaximum(v.Size())},
		}
	} else {
		b.ranges = []bvBoundsRange{{lo: lower, hi: upper}}
	}
	return b
}
