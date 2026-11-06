package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp interval operations.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/math/interval"
)

type bvBoundsRange struct{ lo, hi *big.Int }
type bvBoundsSet struct {
	width  uint32
	ranges []bvBoundsRange
	tight  bool
}

func bvBI(v *big.Int) *big.Int { return new(big.Int).Set(v) }
func bvBoundsSingleton(width uint32, v *big.Int) bvBoundsSet {
	return bvBoundsSet{width: width, ranges: []bvBoundsRange{{bvBI(v), bvBI(v)}}, tight: true}
}
func bvBoundsRangeSet(width uint32, lo, hi *big.Int) bvBoundsSet {
	if lo.Cmp(hi) > 0 {
		return bvBoundsSet{width: width, tight: true}
	}
	return bvBoundsSet{width: width, ranges: []bvBoundsRange{{bvBI(lo), bvBI(hi)}}, tight: true}
}
func bvBoundsLooseRangeSet(width uint32, lo, hi *big.Int) bvBoundsSet {
	b := bvBoundsRangeSet(width, lo, hi)
	b.tight = false
	return b
}
func (b bvBoundsSet) singleton() (*big.Int, bool) {
	v, ok := b.modularInterval()
	if ok && v.IsSingleton() {
		return v.Lower(), true
	}
	return nil, false
}
func (b bvBoundsSet) empty() bool { return len(b.ranges) == 0 }
func (b bvBoundsSet) intersect(o bvBoundsSet) bvBoundsSet {
	if b.empty() || o.empty() || b.width != o.width {
		return bvBoundsSet{width: b.width, tight: b.tight && o.tight}
	}
	a, aok := b.modularInterval()
	z, zok := o.modularInterval()
	if !aok || !zok {
		return bvBoundsSet{width: b.width}
	}
	var result interval.ModularInterval
	used, err := a.Intersect(z, &result)
	if err != nil || !used {
		return bvBoundsSet{width: b.width, tight: b.tight && o.tight}
	}
	return bvBoundsFromModularInterval(&result)
}
func (b bvBoundsSet) normalize() bvBoundsSet {
	if len(b.ranges) < 2 {
		return b
	}
	for i := 0; i < len(b.ranges); i++ {
		for j := i + 1; j < len(b.ranges); j++ {
			if b.ranges[j].lo.Cmp(b.ranges[i].lo) < 0 {
				b.ranges[i], b.ranges[j] = b.ranges[j], b.ranges[i]
			}
		}
	}
	out := bvBoundsSet{width: b.width, tight: b.tight}
	for _, r := range b.ranges {
		n := len(out.ranges)
		if n == 0 || new(big.Int).Add(out.ranges[n-1].hi, big.NewInt(1)).Cmp(r.lo) < 0 {
			out.ranges = append(out.ranges, bvBoundsRange{bvBI(r.lo), bvBI(r.hi)})
			continue
		}
		if r.hi.Cmp(out.ranges[n-1].hi) > 0 {
			out.ranges[n-1].hi = bvBI(r.hi)
		}
	}
	return out
}
func (b bvBoundsSet) implies(o bvBoundsSet) bool {
	if b.width != o.width {
		return false
	}
	if b.empty() {
		return true
	}
	if o.empty() {
		return false
	}
	a, aok := b.modularInterval()
	z, zok := o.modularInterval()
	if !aok || !zok {
		return false
	}
	result, err := a.Implies(z)
	return err == nil && result
}
func (b bvBoundsSet) equal(o bvBoundsSet) bool {
	if b.width != o.width || b.empty() || o.empty() {
		return b.width == o.width && b.empty() && o.empty() && b.tight == o.tight
	}
	a, aok := b.modularInterval()
	z, zok := o.modularInterval()
	if !aok || !zok {
		return false
	}
	result, err := a.Equal(z)
	return err == nil && result
}
func (b bvBoundsSet) complement() bvBoundsSet {
	if b.empty() {
		return bvBoundsRangeSet(b.width, big.NewInt(0), bvBoundsMaximum(b.width))
	}
	v, ok := b.modularInterval()
	if !ok {
		return bvBoundsSet{width: b.width}
	}
	var result interval.ModularInterval
	used, err := v.Negate(&result)
	if err != nil || !used {
		return bvBoundsSet{width: b.width, tight: true}
	}
	return bvBoundsFromModularInterval(&result)
}

// sourceComplement follows bv_bounds_tactic.cpp::interval::negate, whose
// loose intervals complement to the full set and whose tight intervals use
// the source's inclusive endpoints directly.
func (b bvBoundsSet) sourceComplement() bvBoundsSet {
	maximum := bvBoundsMaximum(b.width)
	if !b.tight {
		return bvBoundsRangeSet(b.width, big.NewInt(0), maximum)
	}
	if b.empty() || b.full() {
		return bvBoundsSet{width: b.width, tight: true}
	}
	if len(b.ranges) == 2 {
		// The two stored ranges are the wrapped interval [l..max] U [0..h].
		return bvBoundsRangeSet(b.width,
			new(big.Int).Add(b.ranges[0].hi, big.NewInt(1)),
			new(big.Int).Sub(b.ranges[1].lo, big.NewInt(1)))
	}
	r := b.ranges[0]
	if r.lo.Sign() == 0 {
		return bvBoundsRangeSet(b.width, new(big.Int).Add(r.hi, big.NewInt(1)), maximum)
	}
	if r.hi.Cmp(maximum) == 0 {
		return bvBoundsRangeSet(b.width, big.NewInt(0), new(big.Int).Sub(r.lo, big.NewInt(1)))
	}
	return bvBoundsSet{width: b.width, ranges: []bvBoundsRange{
		{lo: big.NewInt(0), hi: new(big.Int).Sub(r.lo, big.NewInt(1))},
		{lo: new(big.Int).Add(r.hi, big.NewInt(1)), hi: maximum},
	}, tight: true}
}
