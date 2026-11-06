package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp:188-193.

import "math/big"

func (b bvBoundsSet) nonWrappedUpper() (*big.Int, bool) {
	v, ok := b.modularInterval()
	if !ok || v.IsWrapped() {
		return nil, false
	}
	return v.Upper(), true
}
