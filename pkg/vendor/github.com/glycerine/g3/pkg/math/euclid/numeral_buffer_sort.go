package euclid

// Source: src/math/euclid/euclidean_solver.cpp:135-166;
// src/util/numeral_buffer.h:29-89.

import (
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// sortIntCore is the concrete mpz sort_core: IDs are ordered first, then
// each coefficient trades ownership with its indexed buffer slot.
func sortIntCore(as []numeral.Int, xs []uint32, buffer *numeral.ScopedIntVector) error {
	if len(as) != len(xs) {
		return arena.ErrBounds
	}
	m, e := buffer.Manager()
	if e != nil {
		return e
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	for i, x := range xs {
		slot, e := buffer.BorrowSlot(x)
		if e != nil {
			return e
		}
		if e := m.Swap(&as[i], slot); e != nil {
			return e
		}
	}
	return nil
}

// sortInt is the source copy-then-sort path for concrete mpz coefficients.
func sortInt(as []numeral.Int, xs []uint32, buffer *numeral.ScopedIntVector) error {
	if len(as) != len(xs) {
		return arena.ErrBounds
	}
	if _, e := buffer.Manager(); e != nil {
		return e
	}
	for i, x := range xs {
		if e := buffer.Set(x, &as[i]); e != nil {
			return e
		}
	}
	return sortIntCore(as, xs, buffer)
}

func sortRationalCore(as []numeral.Rational, xs []uint32, buffer *numeral.ScopedRationalVector) error {
	if len(as) != len(xs) {
		return arena.ErrBounds
	}
	m, e := buffer.Manager()
	if e != nil {
		return e
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	for i, x := range xs {
		slot, e := buffer.BorrowSlot(x)
		if e != nil {
			return e
		}
		if e := m.Swap(&as[i], slot); e != nil {
			return e
		}
	}
	return nil
}
