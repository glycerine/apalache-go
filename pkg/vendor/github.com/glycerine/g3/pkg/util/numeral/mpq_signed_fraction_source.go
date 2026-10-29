package numeral

// Source: src/util/mpq.h:678-687; src/util/mpz.cpp:560-817.

import (
	"github.com/glycerine/g3/pkg/arena"
	"math"
)

// SetFractionSignedSource preserves the signed32 source overload. Its local
// negations are checked at their actual reach instead of widening them. The
// existing SetFractionSigned service retains its separate widened profile.
func (m *RationalManager) SetFractionSignedSource(a *Rational, n, d int32) error {
	return m.setFractionSignedSourceWithAllocationContext(a, n, d, integerAllocationInvocationContext{})
}
func (m *RationalManager) setFractionSignedSourceWithAllocationContext(a *Rational, n, d int32, c integerAllocationInvocationContext) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := rationalFractionDenominatorAssertion(d != 0, 679); e != nil {
		return e
	}
	if d < 0 {
		if n == math.MinInt32 {
			return arena.ErrBounds
		}
		n = -n
		if d == math.MinInt32 {
			return arena.ErrBounds
		}
		d = -d
	}
	if e := m.Z.setInt64WithAllocationContext(&a.Num, int64(n), c); e != nil {
		return e
	}
	if e := m.Z.setInt64WithAllocationContext(&a.Den, int64(d), c); e != nil {
		return e
	}
	return m.normalizeWithAllocationContext(a, c)
}
