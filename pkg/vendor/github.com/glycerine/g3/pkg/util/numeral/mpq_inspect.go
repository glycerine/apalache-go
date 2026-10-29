package numeral

// Source: src/util/mpq.h/.cpp inspection and thin extraction/setter branches;
// explicit inclusive 32-bit projections come from src/util/rational.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrRationalFloatDomain = errors.New("mpnum: rational converted floating denominator must be positive")

func (m *RationalManager) Precise() bool { return true }
func (m *RationalManager) Field() bool   { return true }

// Sign retains source m_val: small numerators return their entire inline value,
// while big numerators return -1/+1. It is not a normalized mathematical sign.
func (m *RationalManager) Sign(a *Rational) (int32, error) {
	if e := m.checkScalarValue(a); e != nil {
		return 0, e
	}
	return a.Num.Val, nil
}
func (m *RationalManager) IsPos(a *Rational) (bool, error) { v, e := m.Sign(a); return v > 0, e }
func (m *RationalManager) IsNeg(a *Rational) (bool, error) { v, e := m.Sign(a); return v < 0, e }
func (m *RationalManager) IsZero(a *Rational) (bool, error) {
	v, e := m.Sign(a)
	if e != nil {
		return false, e
	}
	return v == 0, nil
}
func (m *RationalManager) IsNonPos(a *Rational) (bool, error) {
	v, e := m.Sign(a)
	if e != nil {
		return false, e
	}
	return v <= 0, nil
}
func (m *RationalManager) IsNonNeg(a *Rational) (bool, error) {
	v, e := m.Sign(a)
	if e != nil {
		return false, e
	}
	return v >= 0, nil
}

// Scalar inspection validates the owner before the source's actual component
// reads. It does not preflight pointees that a structural branch never reads.
func (m *RationalManager) checkScalarValue(a *Rational) error {
	if e := m.checkOwnerReady(); e != nil {
		return e
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	return nil
}
func (m *RationalManager) IsInt(a *Rational) (bool, error) {
	if e := m.checkScalarValue(a); e != nil {
		return false, e
	}
	return isIntegerRational(a), nil
}
func (m *RationalManager) IsOne(a *Rational) (bool, error) {
	if e := m.checkScalarValue(a); e != nil {
		return false, e
	}
	return isOneInteger(&a.Num) && isOneInteger(&a.Den), nil
}
func (m *RationalManager) IsMinusOne(a *Rational) (bool, error) {
	if e := m.checkScalarValue(a); e != nil {
		return false, e
	}
	return a.Num.IsSmall() && a.Num.Val == -1 && isOneInteger(&a.Den), nil
}

// Hash is deliberately numerator-only; fraction collisions and sign-free big
// magnitude hashes are source behavior, resolved by source structural equality.
func (m *RationalManager) Hash(a *Rational) (uint32, error) {
	if e := m.checkScalarValue(a); e != nil {
		return 0, e
	}
	return m.Z.Hash(&a.Num)
}
func (m *RationalManager) IsUint64(a *Rational) (bool, error) {
	if e := m.checkScalarValue(a); e != nil {
		return false, e
	}
	if !isIntegerRational(a) {
		return false, nil
	}
	return m.Z.IsUint64(&a.Num)
}
func (m *RationalManager) IsInt64(a *Rational) (bool, error) {
	if e := m.checkScalarValue(a); e != nil {
		return false, e
	}
	if !isIntegerRational(a) {
		return false, nil
	}
	return m.Z.IsInt64(&a.Num)
}
func (m *RationalManager) Uint64(a *Rational) (uint64, error) {
	if e := m.checkScalarValue(a); e != nil {
		return 0, e
	}
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := m.IsUint64(a)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpq.h", 776, "is_uint64(a)"); e != nil {
					return 0, e
				}
			}
		}
	}
	return m.Z.Uint64(&a.Num)
}
func (m *RationalManager) Int64(a *Rational) (int64, error) {
	if e := m.checkScalarValue(a); e != nil {
		return 0, e
	}
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := m.IsInt64(a)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpq.h", 778, "is_int64(a)"); e != nil {
					return 0, e
				}
			}
		}
	}
	return m.Z.Int64(&a.Num)
}

// IsUnsigned/Unsigned project rational.h's inclusive UINT_MAX boundary onto
// the explicit owner service. MPZ IsUint32/Uint32 instead exclude UINT_MAX.
func (m *RationalManager) IsUnsigned(a *Rational) (bool, error) {
	ok, e := m.IsUint64(a)
	if e != nil || !ok {
		return false, e
	}
	v, e := m.Z.Uint64(&a.Num)
	return v < uint64(1)<<32, e
}
func (m *RationalManager) Unsigned(a *Rational) (uint32, error) {
	if e := m.checkScalarValue(a); e != nil {
		return 0, e
	}
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := m.IsUnsigned(a)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/rational.h", 108, "is_unsigned()"); e != nil {
					return 0, e
				}
			}
		}
	}
	v, e := m.Uint64(a)
	return uint32(v), e
}

// IsInt32/Int32 preserve rational.h's inclusive signed boundaries, unlike the
// source MPZ strict-bound variants. No global-manager wrapper is introduced.
func (m *RationalManager) IsInt32(a *Rational) (bool, error) {
	if e := m.checkScalarValue(a); e != nil {
		return false, e
	}
	if a.IsSmall() && isIntegerRational(a) {
		return true, nil
	}
	ok, e := m.IsInt64(a)
	if e != nil || !ok {
		return false, e
	}
	v, e := m.Z.Int64(&a.Num)
	return v >= -2147483648 && v <= 2147483647, e
}
func (m *RationalManager) Int32(a *Rational) (int32, error) {
	if e := m.checkScalarValue(a); e != nil {
		return 0, e
	}
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := m.IsInt32(a)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/rational.h", 121, "is_int32()"); e != nil {
					return 0, e
				}
			}
		}
	}
	v, e := m.Int64(a)
	return int32(v), e
}
func (m *RationalManager) IsEven(a *Rational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if !isIntegerRational(a) {
		return false, nil
	}
	return m.Z.IsEven(&a.Num)
}
func (m *RationalManager) IsPowerOfTwo(a *Rational) (bool, uint32, error) {
	if e := m.check(a); e != nil {
		return false, 0, e
	}
	if !isIntegerRational(a) {
		return false, 0, nil
	}
	return m.Z.IsPowerOfTwo(&a.Num)
}
func (m *RationalManager) Bitsize(a *Rational) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	n, e := m.Z.Bitsize(&a.Num)
	if e != nil || isIntegerRational(a) {
		return n, e
	}
	d, e := m.Z.Bitsize(&a.Den)
	return n + d, e // source unsigned sum
}
func (m *RationalManager) StorageSize(a *Rational) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	n, e := m.Z.SizeInfo(&a.Num)
	if e != nil {
		return 0, e
	}
	d, e := m.Z.SizeInfo(&a.Den)
	return n + d, e // sorting weight, not owner bytes
}

// Float64 retains each underlying source UINT_MAX-radix conversion and n/d.
// SASSERT(d>0) is selected only in source-debug builds, after both conversions.
// Release retains IEEE quotients for zero, negative and NaN denominators.
func (m *RationalManager) Float64(a *Rational) (float64, error) {
	if e := m.checkScalarValue(a); e != nil {
		return 0, e
	}
	n, e := m.Z.Float64(&a.Num)
	if e != nil {
		return 0, e
	}
	d, e := m.Z.Float64(&a.Den)
	if e != nil {
		return 0, e
	}
	if modularSourceDebug {
		dbg := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(dbg)
		if e != nil {
			return 0, e
		}
		if enabled && !(d > 0) {
			if e := diagnostic.DebugAssertionFailure(dbg, "src/util/mpq.cpp", 302, "d > 0.0"); e != nil {
				return 0, e
			}
		}
	}
	return n / d, nil
}

// Extraction copies owned payloads explicitly and does not normalize either
// source or result. Component output aliases follow the source Copy action.
func (m *RationalManager) Numerator(a *Rational, out *Int) error {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	return m.Z.Copy(out, &a.Num)
}
func (m *RationalManager) Denominator(a *Rational, out *Int) error {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	return m.Z.Copy(out, &a.Den)
}
func (m *RationalManager) NumeratorRational(a, out *Rational) error {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	if e := m.checkScalarValue(out); e != nil {
		return e
	}
	if e := m.Z.Copy(&out.Num, &a.Num); e != nil {
		return e
	}
	return m.resetDenominator(out)
}
func (m *RationalManager) DenominatorRational(a, out *Rational) error {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	if e := m.checkScalarValue(out); e != nil {
		return e
	}
	if e := m.Z.Copy(&out.Num, &a.Den); e != nil {
		return e
	}
	return m.resetDenominator(out)
}

// SetInteger and the thin setters publish source numerator actions followed by
// reset_denominator. Errors retain actual Integer publication prefixes and leave
// DEN untouched if NUM fails. No borrowed limb slice remains.
// SetIntegerWithCopyAllocationReceipt preserves NUM SET before DEN reset and
// records only that actual numerator copy's allocation.
func (m *RationalManager) SetIntegerWithCopyAllocationReceipt(a *Rational, n *Int, receipt *IntegerCopyAllocationReceipt) error {
	return m.setIntegerFromOwnerWithReceiptAndAllocationContext(a, m.Z, n, receipt, integerAllocationInvocationContext{})
}
func (m *RationalManager) SetInteger(a *Rational, n *Int) error {
	return m.setIntegerWithAllocationContext(a, n, integerAllocationInvocationContext{})
}

func (m *RationalManager) setIntegerWithAllocationContext(a *Rational, n *Int, context integerAllocationInvocationContext) error {
	return m.setIntegerFromOwnerWithAllocationContext(a, m.Z, n, context)
}
func (m *RationalManager) setIntegerFromOwnerWithAllocationContext(a *Rational, source *IntegerManager, n *Int, context integerAllocationInvocationContext) error {
	return m.setIntegerFromOwnerWithReceiptAndAllocationContext(a, source, n, nil, context)
}

func (m *RationalManager) setIntegerFromOwnerWithReceiptAndAllocationContext(a *Rational, source *IntegerManager, n *Int, receipt *integerCopyAllocationReceipt, context integerAllocationInvocationContext) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := m.Z.copyFromOwnerWithAllocationContext(&a.Num, source, n, receipt, context); e != nil {
		return e
	}
	return m.resetDenominator(a)
}

// Source: src/util/mpq.h:712-715. NUM publication precedes DEN reset.
func (m *RationalManager) SetUnsigned(a *Rational, n uint32) error {
	return m.setUnsignedWithAllocationContext(a, n, integerAllocationInvocationContext{})
}
func (m *RationalManager) setUnsignedWithAllocationContext(a *Rational, n uint32, context integerAllocationInvocationContext) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := m.Z.setUint32WithAllocationContext(&a.Num, n, context); e != nil {
		return e
	}
	return m.resetDenominator(a)
}
func (m *RationalManager) SetUint64(a *Rational, n uint64) error {
	return m.setUint64WithAllocationContext(a, n, integerAllocationInvocationContext{})
}
func (m *RationalManager) setUint64WithAllocationContext(a *Rational, n uint64, context integerAllocationInvocationContext) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := m.Z.setUint64WithAllocationContext(&a.Num, n, context); e != nil {
		return e
	}
	return m.resetDenominator(a)
}
func (m *RationalManager) SetLimbs(a *Rational, digits []uint32) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := m.Z.SetLimbs(&a.Num, digits); e != nil {
		return e
	}
	return m.resetDenominator(a)
}
func (m *RationalManager) SwapNumerator(n *Int, a *Rational) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.Z.Swap(n, &a.Num)
}
func (m *RationalManager) Inc(a *Rational) error { one := NewInt(1); return m.AddInt(a, &one, a) }
func (m *RationalManager) Dec(a *Rational) error {
	minusOne := NewInt(-1)
	return m.AddInt(a, &minusOne, a)
}
