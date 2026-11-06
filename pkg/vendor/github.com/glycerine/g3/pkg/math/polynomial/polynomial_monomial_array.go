package polynomial

// Source: src/math/polynomial/polynomial.cpp:2217-2245,3168-3180; src/util/buffer.h.

import "github.com/glycerine/g3/pkg/arena"

// A borrowed span of actual monomial handles. This view owns no references.
type monomialArrayView struct {
	values []Monomial
	count  uint32
}

func (v monomialArrayView) at(i uint32) (Monomial, error) {
	if i >= v.count || uint64(i) >= uint64(len(v.values)) {
		return Monomial{}, arena.ErrBounds
	}
	return v.values[i], nil
}
