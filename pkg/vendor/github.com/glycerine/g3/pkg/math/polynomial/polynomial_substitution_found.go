package polynomial

// Source: src/math/polynomial/polynomial.cpp:3185,6253-6298.

import "github.com/glycerine/g3/pkg/arena"

// Both substitution and Vars retain flags until their reached reset loops.
// Growth initializes only new flags; reserving fewer variables does not shrink.
type polynomialFoundVariables struct {
	values []bool
	closed bool
}

func (v *polynomialFoundVariables) reserve(n uint32) error {
	if v.closed {
		return arena.ErrClosed
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(n) > len(v.values) {
		v.values = append(v.values, make([]bool, int(n)-len(v.values))...)
	}
	return nil
}
func (v *polynomialFoundVariables) at(x uint32) (bool, error) {
	if v.closed {
		return false, arena.ErrClosed
	}
	if uint64(x) >= uint64(len(v.values)) {
		if polynomialSourceDebug {
			return false, ErrPolynomialAssertion
		}
		return false, arena.ErrBounds
	}
	return v.values[x], nil
}
func (v *polynomialFoundVariables) put(x uint32, b bool) error {
	if v.closed {
		return arena.ErrClosed
	}
	if uint64(x) >= uint64(len(v.values)) {
		if polynomialSourceDebug {
			return ErrPolynomialAssertion
		}
		return arena.ErrBounds
	}
	v.values[x] = b
	return nil
}
func (v *polynomialFoundVariables) sourceFinalize() error { return v.close() }
func (v *polynomialFoundVariables) close() error {
	v.values = nil
	v.closed = true
	return nil
}
