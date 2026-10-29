package numeral

// Source: src/util/mpq.cpp root and mpq.h square predicates/output wrappers.

import "github.com/glycerine/g3/pkg/util/diagnostic"

// Root follows the source direct caller order: the numerator Root publishes
// into out.Num before the denominator Root is considered. An inexact numerator
// leaves the old output denominator untouched; a failing call keeps its actual
// integer publication prefix, including when input and output alias.
func (m *RationalManager) Root(a *Rational, n uint32, out *Rational) (bool, error) {
	return m.RootWithInvocation(a, n, out, nil)
}
func (m *RationalManager) RootWithInvocation(a *Rational, n uint32, out *Rational, receipt *RationalRootInvocation) (bool, error) {
	if receipt != nil && receipt.Pending() {
		return false, ErrRootPending
	}
	if e := m.check(a); e != nil {
		return false, e
	}
	if e := m.check(out); e != nil {
		return false, e
	}
	var exact bool
	var e error
	if receipt == nil {
		exact, e = m.Z.Root(&a.Num, n, &out.Num)
	} else {
		*receipt = RationalRootInvocation{manager: m}
		exact, e = m.Z.rootWithAllocationContext(&a.Num, n, &out.Num, &receipt.copies[0], &receipt.roots[0], integerAllocationInvocationContext{})
	}
	if e != nil {
		return false, e
	}
	if !exact {
		return false, nil
	}
	if receipt == nil {
		return m.Z.Root(&a.Den, n, &out.Den)
	}
	return m.Z.rootWithAllocationContext(&a.Den, n, &out.Den, &receipt.copies[1], &receipt.roots[1], integerAllocationInvocationContext{})
}

// IsPerfectSquare writes directly to the caller's rational in source order.
// Only a fully precise fractional result is normalized.
func (m *RationalManager) IsPerfectSquare(a, out *Rational) (bool, error) {
	return m.perfectSquare(a, out, false)
}
func (m *RationalManager) IsIntPerfectSquare(a, out *Rational) (bool, error) {
	return m.perfectSquare(a, out, true)
}
func (m *RationalManager) perfectSquare(a, out *Rational, integerOnly bool) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if e := m.check(out); e != nil {
		return false, e
	}
	integer := isIntegerRational(a)
	if integerOnly {
		if modularSourceDebug {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, e := diagnostic.SourceAssertionsEnabled(d)
			if e != nil {
				return false, e
			}
			if enabled && !integer {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpq.h", 839, "is_int(a)"); e != nil {
					return false, e
				}
			}
		}
		if e := m.resetDenominator(out); e != nil {
			return false, e
		}
		return m.Z.IsPerfectSquare(&a.Num, &out.Num)
	}
	if integer {
		if e := m.resetDenominator(out); e != nil {
			return false, e
		}
		return m.Z.IsPerfectSquare(&a.Num, &out.Num)
	}
	exact, e := m.Z.IsPerfectSquare(&a.Num, &out.Num)
	if e != nil {
		return false, e
	}
	if !exact {
		return false, nil
	}
	exact, e = m.Z.IsPerfectSquare(&a.Den, &out.Den)
	if e != nil {
		return false, e
	}
	if !exact {
		return false, nil
	}
	if e := m.normalize(out); e != nil {
		return false, e
	}
	return true, nil
}
