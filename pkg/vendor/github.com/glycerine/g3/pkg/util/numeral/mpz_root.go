package numeral

// Source: src/util/mpz.cpp; src/util/mpz.h (root/is_perfect_square and copy wrapper).

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrRootDegreeZero = errors.New("mpnum: nonzero integer root with degree zero divides by zero in source")

type RootDomainReason uint8

const (
	RootZeroDegree RootDomainReason = iota + 1
)

// RootDomainError guards host-undefined zero-degree division.
type RootDomainError struct {
	Degree uint32
	Reason RootDomainReason
}

func (e *RootDomainError) Error() string { return e.Unwrap().Error() }
func (e *RootDomainError) Unwrap() error {
	return ErrRootDegreeZero
}
func rootDomain(a *Int, n uint32) error {
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && n%2 == 0 && a.Val < 0 {
			if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1943, "n % 2 != 0 || is_nonneg(a)"); e != nil {
				return e
			}
		}
	}
	if a.Val != 0 && n == 0 {
		return &RootDomainError{Degree: n, Reason: RootZeroDegree}
	}
	return nil
}

// Root is literal source SET(out,a) followed by in-place root. Caller output
// publication precedes domain checks and every bisection/body failure.
func (m *IntegerManager) Root(a *Int, n uint32, out *Int) (bool, error) {
	if e := m.Copy(out, a); e != nil {
		return false, e
	}
	return m.RootInPlace(out, n)
}
func (m *IntegerManager) RootInPlace(a *Int, n uint32) (bool, error) {
	return m.rootInPlaceWithReceipt(a, n, nil)
}
func (m *IntegerManager) rootInPlaceWithReceipt(a *Int, n uint32, receipt *integerRootInvocationReceipt) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if e := rootDomain(a, n); e != nil {
		return false, e
	}
	return m.rootCoreWithReceipt(a, n, receipt)
}
func (m *IntegerManager) rootCore(a *Int, n uint32) (bool, error) {
	return m.rootCoreWithReceipt(a, n, nil)
}
func (m *IntegerManager) rootWithAllocationContext(a *Int, n uint32, out *Int, copyReceipt *integerCopyAllocationReceipt, receipt *integerRootInvocationReceipt, context integerAllocationInvocationContext) (bool, error) {
	if context != (integerAllocationInvocationContext{}) && context.roots(m) == nil {
		return false, ErrNumericOwner
	}
	if e := m.copyFromOwnerWithAllocationContext(out, m, a, copyReceipt, context); e != nil {
		return false, e
	}
	if e := m.check(out); e != nil {
		return false, e
	}
	if e := rootDomain(out, n); e != nil {
		return false, e
	}
	return m.rootCoreWithReceiptAndContext(out, n, receipt, context)
}

func (m *IntegerManager) rootCoreWithReceipt(a *Int, n uint32, receipt *integerRootInvocationReceipt) (bool, error) {
	return m.rootCoreWithReceiptAndContext(a, n, receipt, integerAllocationInvocationContext{})
}
func (m *IntegerManager) rootCoreWithReceiptAndContext(a *Int, n uint32, receipt *integerRootInvocationReceipt, context integerAllocationInvocationContext) (exact bool, cause error) {
	if context != (integerAllocationInvocationContext{}) && context.roots(m) == nil {
		return false, ErrNumericOwner
	}

	if a.Val == 0 {
		return true, nil
	}
	if e := m.beginRootRawScopeWithReceipt(receipt); e != nil {
		return false, e
	}
	s := &m.RootRaw
	defer func() {
		if cause != nil && !s.EndEntered && s.bodyCause == nil {
			s.bodyCause = cause
		}
	}()
	lower, upper, mid, midN := &s.Values[0], &s.Values[1], &s.Values[2], &s.Values[3]
	var k uint32
	var e error
	if a.Val > 0 {
		k, e = m.Log2(a)
	} else {
		k, e = m.MLog2(a)
	}
	if e != nil {
		return false, e
	}
	l, u := k/n, (k+1)/n
	if (k+1)%n != 0 {
		u++
	}
	two := NewInt(2)
	loExp, hiExp := NewInt(int32(l)), NewInt(int32(u))
	if a.Val > 0 {
		if e = m.rootRawCallWithContext(0, &two, &loExp, lower, context); e != nil {
			return false, e
		}
		if e = m.rootRawCallWithContext(0, &two, &hiExp, upper, context); e != nil {
			return false, e
		}
	} else {
		if e = m.rootRawCallWithContext(0, &two, &hiExp, lower, context); e != nil {
			return false, e
		}
		if e = m.rootRawCallWithContext(0, &two, &loExp, upper, context); e != nil {
			return false, e
		}
		if e = m.Neg(lower); e != nil {
			return false, e
		}
		if e = m.Neg(upper); e != nil {
			return false, e
		}
	}
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return false, e
		}
		if enabled {
			le, e := m.LessEqual(lower, upper)
			if e != nil {
				return false, e
			}
			if !le {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1977, "le(lower, upper)"); e != nil {
					return false, e
				}
			}
		}
	}
	equal, e := m.Equal(lower, upper)
	if e != nil {
		return false, e
	}
	if equal {
		if e = m.Swap(a, lower); e != nil {
			return false, e
		}
		return true, m.endRootRawScope()
	}
	for {
		if e = m.rootRawCallWithContext(1, upper, lower, mid, context); e != nil {
			return false, e
		}
		one := NewInt(1)
		if e = m.rootRawCallWithContext(2, nil, &one, mid, context); e != nil {
			return false, e
		}
		m.traceRootBisection(upper, lower, mid)
		power := NewInt(int32(n))
		if e = m.rootRawCallWithContext(0, mid, &power, midN, context); e != nil {
			return false, e
		}
		equal, e = m.Equal(midN, a)
		if e != nil {
			return false, e
		}
		if equal {
			if e = m.Swap(a, mid); e != nil {
				return false, e
			}
			return true, m.endRootRawScope()
		}
		atLower, e := m.Equal(mid, lower)
		if e != nil {
			return false, e
		}
		atEndpoint := atLower
		if !atEndpoint {
			atEndpoint, e = m.Equal(mid, upper)
			if e != nil {
				return false, e
			}
		}
		if atEndpoint {
			if e = m.Swap(a, upper); e != nil {
				return false, e
			}
			return false, m.endRootRawScope()
		}
		less, e := m.Less(midN, a)
		if e != nil {
			return false, e
		}
		if less {
			e = m.Swap(mid, lower)
		} else {
			if modularSourceDebug {
				d := diagnostic.BorrowProcessDebugContext()
				enabled, e := diagnostic.SourceAssertionsEnabled(d)
				if e != nil {
					return false, e
				}
				if enabled {
					less, e := m.Less(a, midN)
					if e != nil {
						return false, e
					}
					if !less {
						if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 2004, "lt(a, mid_n)"); e != nil {
							return false, e
						}
					}
				}
			}
			e = m.Swap(mid, upper)
		}
		if e != nil {
			return false, e
		}
	}
}

// IsPerfectSquare preserves source false-output conventions and its alias
// defect: resetting a nonnegative aliased output makes the input zero, so the
// result is true with output zero. Negative inputs leave out unchanged.
func (m *IntegerManager) IsPerfectSquare(a, out *Int) (bool, error) {
	return m.isPerfectSquareWithAllocationContext(a, out, integerAllocationInvocationContext{})
}
func (m *IntegerManager) isPerfectSquareWithAllocationContext(a, out *Int, context integerAllocationInvocationContext) (bool, error) {
	return m.isPerfectSquareTracked(a, out, context, nil)
}
func (m *IntegerManager) isPerfectSquareTracked(a, out *Int, context integerAllocationInvocationContext, receipt *integerPerfectSquareInvocationReceipt) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if e := m.check(out); e != nil {
		return false, e
	}
	if a.Val < 0 {
		return false, nil
	}
	return m.squareCoreTracked(a, out, context, receipt)
}

func (m *IntegerManager) squareCore(a, out *Int) (bool, error) {
	return m.squareCoreWithContext(a, out, integerAllocationInvocationContext{})
}
func (m *IntegerManager) squareCoreWithContext(a, out *Int, context integerAllocationInvocationContext) (exact bool, cause error) {
	return m.squareCoreTracked(a, out, context, nil)
}
func (m *IntegerManager) squareCoreTracked(a, out *Int, context integerAllocationInvocationContext, receipt *integerPerfectSquareInvocationReceipt) (exact bool, cause error) {
	if context != (integerAllocationInvocationContext{}) && context.roots(m) == nil {
		return false, ErrNumericPending
	}
	if e := m.Reset(out); e != nil {
		return false, e
	}
	if a.Val == 0 {
		return true, nil
	}
	one := NewInt(1)
	same, e := m.IsOne(a)
	if e != nil {
		return false, e
	}
	if same {
		return true, m.setInt64WithAllocationContext(out, 1, context)
	}
	if e = m.beginPerfectSquareRawScopeWithContext(context); e != nil {
		return false, e
	}
	s := &m.PerfectSquare
	if receipt != nil {
		s.directReceipt = integerPerfectSquareInvocationReceipt{m, s, s.Generation}
		*receipt = s.directReceipt
	}
	if receipt != nil || context != (integerAllocationInvocationContext{}) {
		s.Executing = true
		defer func() { s.Executing = false }()
	}
	childContext := context
	if context != (integerAllocationInvocationContext{}) {
		childContext.perfectSquare = &s.directReceipt
		childContext.perfectSquareInvocation = s.Generation
	}
	defer func() {
		if cause != nil && !s.EndEntered && s.BodyCause == nil {
			s.BodyCause = cause
		}
	}()
	lo, hi, mid, sqLo, sqHi, sqMid := &s.Values[0], &s.Values[1], &s.Values[2], &s.Values[3], &s.Values[4], &s.Values[5]
	if e = m.setInt64WithAllocationContext(lo, 1, childContext); e != nil {
		return false, e
	}
	if e = m.perfectSquareCallWithContext(0, a, nil, hi, childContext); e != nil {
		return false, e
	}
	if e = m.setInt64WithAllocationContext(sqLo, 1, childContext); e != nil {
		return false, e
	}
	if e = m.perfectSquareCallWithContext(2, hi, hi, sqHi, childContext); e != nil {
		return false, e
	}
	two := NewInt(2)
	for {
		if modularSourceDebug {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, x := diagnostic.SourceAssertionsEnabled(d)
			if x != nil {
				return false, x
			}
			if enabled {
				less, x := m.Less(lo, hi)
				if x != nil {
					return false, x
				}
				if !less {
					if x = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1891, "lt(lo, hi)"); x != nil {
						return false, x
					}
				}
			}
			enabled, x = diagnostic.SourceAssertionsEnabled(d)
			if x != nil {
				return false, x
			}
			if enabled {
				bounds, x := m.LessEqual(sqLo, a)
				if x != nil {
					return false, x
				}
				if bounds {
					bounds, x = m.Less(a, sqHi)
					if x != nil {
						return false, x
					}
				}
				if !bounds {
					if x = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1892, "le(sq_lo, a) && lt(a, sq_hi)"); x != nil {
						return false, x
					}
				}
			}
		}
		same, e = m.Equal(sqLo, a)
		if e != nil {
			return false, e
		}
		if same {
			s.outputStarted = true
			s.outputChoice = 0
			if e = m.copyFromOwnerWithAllocationContext(out, m, lo, nil, context); e != nil {
				return false, e
			}
			exact = true
			break
		}
		if e = m.perfectSquareCallWithContext(1, lo, &one, mid, childContext); e != nil {
			return false, e
		}
		same, e = m.Equal(mid, hi)
		if e != nil {
			return false, e
		}
		if same {
			s.outputStarted = true
			s.outputChoice = 1
			if e = m.copyFromOwnerWithAllocationContext(out, m, hi, nil, context); e != nil {
				return false, e
			}
			exact = false
			break
		}
		if e = m.perfectSquareCallWithContext(1, hi, lo, mid, childContext); e != nil {
			return false, e
		}
		if e = m.perfectSquareCallWithContext(3, mid, &two, mid, childContext); e != nil {
			return false, e
		}
		if modularSourceDebug {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, x := diagnostic.SourceAssertionsEnabled(d)
			if x != nil {
				return false, x
			}
			if enabled {
				bounds, x := m.Less(lo, mid)
				if x != nil {
					return false, x
				}
				if bounds {
					bounds, x = m.Less(mid, hi)
					if x != nil {
						return false, x
					}
				}
				if !bounds {
					if x = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1910, "lt(lo, mid) && lt(mid, hi)"); x != nil {
						return false, x
					}
				}
			}
		}
		if e = m.perfectSquareCallWithContext(2, mid, mid, sqMid, childContext); e != nil {
			return false, e
		}
		cmp, x := m.Compare(sqMid, a)
		if x != nil {
			return false, x
		}
		if cmp > 0 {
			if e = m.perfectSquareCallWithContext(0, mid, nil, hi, childContext); e != nil {
				return false, e
			}
			if e = m.perfectSquareCallWithContext(0, sqMid, nil, sqHi, childContext); e != nil {
				return false, e
			}
		} else {
			if e = m.perfectSquareCallWithContext(0, mid, nil, lo, childContext); e != nil {
				return false, e
			}
			if e = m.perfectSquareCallWithContext(0, sqMid, nil, sqLo, childContext); e != nil {
				return false, e
			}
		}
	}
	if e = m.endPerfectSquareRawScope(); e != nil {
		return false, e
	}
	return exact, nil
}
