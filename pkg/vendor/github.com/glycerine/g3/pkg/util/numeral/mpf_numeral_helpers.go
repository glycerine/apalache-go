package numeral

// Source: src/util/mpf.h:75,181; src/util/mpf.cpp:1890-1896,2133-2141.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (m *ArbitraryFloatManager) Reset(out *ArbitraryFloat, ebits, sbits uint32) error {
	return m.SetInt32(out, ebits, sbits, 0)
}

// MkOne publishes the sign before SET, and the exponent only after SET.
// Unlike MkZero, it does not read or populate exponent-bound caches.
func (m *ArbitraryFloatManager) MkOne(ebits, sbits uint32, sign bool, out *ArbitraryFloat) error {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	out.assignSbits(sbits)
	out.assignEbits(ebits)
	out.assignSign(sign)
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.integer.SetInt32(&out.significand, 0); e != nil {
		return e
	}
	out.exponent = 0
	return nil
}

// IsRegular retains the literal source policy: formatted signed zero is
// neither normal nor denormal. The default sbits=0 branch reads no numeral
// payload or exponent bounds.
func (m *ArbitraryFloatManager) IsRegular(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	if a.SBits() == 0 {
		return true, nil
	}
	normal, e := m.IsNormal(a)
	if e != nil || normal {
		return normal, e
	}
	return m.IsDenormal(a)
}

// PrevPowerOfTwo deliberately implements sbits+exponent-1 from the source,
// including its positive-zero and release infinity behavior. This is not a
// substituted logarithm of the represented value.
func (m *ArbitraryFloatManager) PrevPowerOfTwo(a *ArbitraryFloat) (uint32, error) {
	if e := m.valueGuard(a); e != nil {
		return 0, e
	}
	assertionsEnabled, e := mpfSourceAssertionsEnabled()
	if e != nil {
		return 0, e
	}
	if assertionsEnabled {
		nan, e := m.IsNaN(a)
		if e != nil {
			return 0, e
		}
		positiveInf := false
		if !nan {
			positiveInf, e = m.IsPositiveInf(a)
			if e != nil {
				return 0, e
			}
		}
		negativeInf := false
		if !nan && !positiveInf {
			negativeInf, e = m.IsNegativeInf(a)
			if e != nil {
				return 0, e
			}
		}
		if e := mpfSourceAssertion(!nan && !positiveInf && !negativeInf, 2134, "!is_nan(a) && !is_pinf(a) && !is_ninf(a)"); e != nil {
			return 0, e
		}
	}
	positive, e := m.IsPositive(a)
	if e != nil || !positive {
		return 0, e
	}
	if a.exponent <= -int64(a.SBits()) {
		return 0, nil
	}
	// Source evaluates the signed addition before subtracting one. Diagnose
	// its undefined overflow only when that expression is actually reached.
	if a.exponent > math.MaxInt64-int64(a.SBits()) {
		return 0, arena.ErrBounds
	}
	return uint32(int64(a.SBits()) + a.exponent - 1), nil
}
