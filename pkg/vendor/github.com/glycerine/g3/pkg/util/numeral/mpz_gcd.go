package numeral

// Source: src/util/mpz.cpp:49-84,560-817 (_MP_INTERNAL / LEHMER_GCD);
// src/util/mpz.h:393-426 addmul/submul.

import (
	"math"
)

// U32GCD and U64GCD are concrete translations of the source binary gcd_core.
// The source template is specialized explicitly to obey the no-generics rule.
func U32GCD(u, v uint32) uint32 {
	if u == 0 {
		return v
	}
	if v == 0 {
		return u
	}
	var k uint
	for (u|v)&1 == 0 {
		u >>= 1
		v >>= 1
		k++
	}
	for u&1 == 0 {
		u >>= 1
	}
	for {
		for v&1 == 0 {
			v >>= 1
		}
		if u < v {
			v -= u
		} else {
			diff := u - v
			u = v
			v = diff
		}
		v >>= 1
		if v == 0 {
			break
		}
	}
	return u << k
}
func U64GCD(u, v uint64) uint64 {
	if u == 0 {
		return v
	}
	if v == 0 {
		return u
	}
	var k uint
	for (u|v)&1 == 0 {
		u >>= 1
		v >>= 1
		k++
	}
	for u&1 == 0 {
		u >>= 1
	}
	for {
		for v&1 == 0 {
			v >>= 1
		}
		if u < v {
			v -= u
		} else {
			diff := u - v
			u = v
			v = diff
		}
		v >>= 1
		if v == 0 {
			break
		}
	}
	return u << k
}

// AddMul stores a+b*c. Only inline +/-1 take the source shortcut paths.
func (m *IntegerManager) AddMul(a, b, c, d *Int) error {
	return m.addMulWithAllocationContext(a, b, c, d, nil, integerAllocationInvocationContext{})
}

func (m *IntegerManager) addMulWithAllocationContext(a, b, c, d *Int, receipt *integerAddMulInvocationReceipt, context integerAllocationInvocationContext) error {
	return m.addSubMulWithAllocationContext(a, b, c, d, false, receipt, context)
}

// Both literal bodies construct one RAW tmp, perform Mul then Add/Sub, and
// execute DEL only after the body succeeds. Failed bodies retain actual roots.
func (m *IntegerManager) addSubMulWithAllocationContext(a, b, c, d *Int, subtract bool, receipt *integerAddMulInvocationReceipt, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Int{a, b, c, d} {
		if e := m.check(n); e != nil {
			return e
		}
	}
	if b.IsSmall() && b.Val == 1 {
		return m.addSubWithAllocationContext(a, c, d, subtract, context)
	}
	if b.IsSmall() && b.Val == -1 {
		return m.addSubWithAllocationContext(a, c, d, !subtract, context)
	}
	callerContext := context
	frame := &m.AddMulInvocation
	if frame.Active || frame.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	if receipt == nil {
		receipt = &frame.Receipt
	}
	*frame = integerAddMulScope{Active: true, executing: true, invocation: frame.invocation + 1, Stage: 1, parentContext: callerContext}
	frame.Receipt = integerAddMulInvocationReceipt{m, frame, frame.invocation}
	*receipt = frame.Receipt
	tmp := &frame.Tmp
	context = (integerAllocationInvocationContext{}).withAddMulInvocation(receipt)
	defer func() {
		frame.executing = false
		if frame.Stage < 3 {
			frame.BodyCause = err
		}
		if tmp.IsSmall() && !NumericAllocationPending(&frame.Numeric) && frame.SourceCause == nil {
			frame.Active = false
		}
	}()
	if e := m.mulWithAllocationContext(b, c, tmp, context); e != nil {
		return e
	}
	frame.Stage = 2
	if callerContext != (integerAllocationInvocationContext{}) {
		context = callerContext
	}
	if e := m.addSubWithAllocationContext(a, tmp, d, subtract, context); e != nil {
		return e
	}
	frame.Stage = 3
	return m.finishAddMulSource(frame)
}

// SubMul stores a-b*c with the corresponding source shortcut branches.
func (m *IntegerManager) SubMul(a, b, c, d *Int) error {
	return m.addSubMulWithAllocationContext(a, b, c, d, true, nil, integerAllocationInvocationContext{})
}

// GCD translates the internal backend's selected Lehmer algorithm, not the
// inactive Euclid/binary alternatives. It returns the nonnegative gcd, including
// gcd(0,0)=0. Destination may alias either input. All working Ints are explicit
// owner-local source copies. The zero fastpath Copies and Abs directly into the
// caller; errors preserve completed actions rather than a staged destination.
func (m *IntegerManager) GCD(a, b, c *Int) error {
	return m.gcdWithAllocationContext(a, b, c, integerAllocationInvocationContext{})
}
func (m *IntegerManager) gcdWithAllocationContext(a, b, c *Int, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Int{a, b, c} {
		if e := m.check(n); e != nil {
			return e
		}
	}
	if a.IsSmall() && b.IsSmall() && a.Val != math.MinInt32 && b.Val != math.MinInt32 {
		av, bv := a.Val, b.Val
		if av < 0 {
			av = -av
		}
		if bv < 0 {
			bv = -bv
		}
		return m.setInt64WithAllocationContext(c, int64(U32GCD(uint32(av), uint32(bv))), context)
	}
	if a.Val == 0 {
		if e := m.copyFromOwnerWithAllocationContext(c, m, b, nil, context); e != nil {
			return e
		}
		return m.absWithAllocationContext(c, context)
	}
	if b.Val == 0 {
		if e := m.copyFromOwnerWithAllocationContext(c, m, a, nil, context); e != nil {
			return e
		}
		return m.absWithAllocationContext(c, context)
	}
	s := &m.GcdRaw
	receipt, e := m.beginRawInvocation(s, 5)
	if e != nil {
		return e
	}
	context.recordRationalSetRaw(m, receipt)
	context.recordRationalServiceRaw(m, receipt)
	context.recordConversionRaw(m, receipt)
	context.recordFormatRaw(m, receipt)
	context.recordCaller(m, "gcd", NumericInvocationReceipt{Raw: receipt})
	context = context.withRawInvocation(receipt)
	a1, b1, r, t, tmp := &s.Values[0], &s.Values[1], &s.Values[2], &s.Values[3], &s.Values[4]
	defer func() {
		s.executing = false
		s.BodyCause = err
		if child := m.AddMulInvocationSourceCause(&s.AddMul); child != nil {
			s.sourceCause = child
		}

		if err == nil {
			s.BodySucceeded = true
			err = m.finishRawSource(s)
		}
	}()
	if e := m.copyFromOwnerWithAllocationContext(a1, m, a, nil, context); e != nil {
		return e
	}
	if e := m.copyFromOwnerWithAllocationContext(b1, m, b, nil, context); e != nil {
		return e
	}
	if e := m.absWithAllocationContext(a1, context); e != nil {
		return e
	}
	if e := m.absWithAllocationContext(b1, context); e != nil {
		return e
	}
	cmp, e := m.Compare(a1, b1)
	if e != nil {
		return e
	}
	if cmp < 0 {
		if e = m.Swap(a1, b1); e != nil {
			return e
		}
	}
	for {
		if e = m.gcdSourceGreaterEqualAt(a1, b1, 738, "ge(a1, b1)"); e != nil {
			return e
		}
		if b1.IsSmall() {
			if a1.IsSmall() {
				return m.setInt64WithAllocationContext(c, int64(U32GCD(uint32(a1.Val), uint32(b1.Val))), context)
			}
			for b1.Val != 0 {
				if e = m.gcdSourceGreaterEqualAt(a1, b1, 747, "ge(a1, b1)"); e != nil {
					return e
				}
				if e = m.remWithAllocationContext(a1, b1, tmp, context); e != nil {
					return e
				}
				if e = m.Swap(a1, b1); e != nil {
					return e
				}
				if e = m.Swap(b1, tmp); e != nil {
					return e
				}
			}
			return m.Swap(c, a1)
		}
		if sourceGCDDebug {
			if e = gcdSourceRequireAt(!a1.IsSmall(), 756, "!is_small(a1)"); e != nil {
				return e
			}
			if e = gcdSourceRequireAt(!b1.IsSmall(), 757, "!is_small(b1)"); e != nil {
				return e
			}
		}
		asize, bsize := m.Cell(a1.Cell).Size, m.Cell(b1.Cell).Size
		if sourceGCDDebug {
			if e = gcdSourceRequireAt(bsize <= asize, 760, "b_sz <= a_sz"); e != nil {
				return e
			}
		}
		ahat := int64(m.Digit(a1.Cell, asize-1))
		bhat := int64(0)
		if bsize == asize {
			bhat = int64(m.Digit(b1.Cell, bsize-1))
		}
		A, B, C, D := int64(1), int64(0), int64(0), int64(1)
		for {
			if sourceGCDDebug {
				limit := int64(math.MaxUint32) + 1
				if e = gcdSourceRequireAt(ahat+A <= limit, 769, "a_hat + A <= static_cast<int64_t>(UINT_MAX) + 1"); e != nil {
					return e
				}
				if e = gcdSourceRequireAt(ahat+B < limit, 770, "a_hat + B < static_cast<int64_t>(UINT_MAX) + 1"); e != nil {
					return e
				}
				if e = gcdSourceRequireAt(bhat+C < limit, 771, "b_hat + C < static_cast<int64_t>(UINT_MAX) + 1"); e != nil {
					return e
				}
				if e = gcdSourceRequireAt(bhat+D <= limit, 772, "b_hat + D <= static_cast<int64_t>(UINT_MAX) + 1"); e != nil {
					return e
				}
			}
			if bhat+C == 0 || bhat+D == 0 {
				break
			}
			quotient := (ahat + A) / (bhat + C)
			if quotient != (ahat+B)/(bhat+D) {
				break
			}
			T := A - quotient*C
			A = C
			C = T
			T = B - quotient*D
			B = D
			D = T
			T = ahat - quotient*bhat
			ahat = bhat
			bhat = T
		}
		if e = m.gcdSourceGreaterEqualAt(a1, b1, 789, "ge(a1, b1)"); e != nil {
			return e
		}
		if B == 0 {
			if e = m.remWithAllocationContext(a1, b1, t, context); e != nil {
				return e
			}
			if e = m.Swap(a1, b1); e != nil {
				return e
			}
			if e = m.Swap(b1, t); e != nil {
				return e
			}
			if e = m.gcdSourceGreaterEqualAt(a1, b1, 794, "ge(a1, b1)"); e != nil {
				return e
			}
		} else {
			if e = m.setInt64WithAllocationContext(tmp, A, context); e != nil {
				return e
			}
			if e = m.mulWithAllocationContext(a1, tmp, t, context); e != nil {
				return e
			}
			if e = m.setInt64WithAllocationContext(tmp, B, context); e != nil {
				return e
			}
			if e = m.addMulWithAllocationContext(t, tmp, b1, t, &s.AddMul, context); e != nil {
				return e
			}
			if e = m.setInt64WithAllocationContext(tmp, C, context); e != nil {
				return e
			}
			if e = m.mulWithAllocationContext(a1, tmp, r, context); e != nil {
				return e
			}
			if e = m.setInt64WithAllocationContext(tmp, D, context); e != nil {
				return e
			}
			if e = m.addMulWithAllocationContext(r, tmp, b1, r, &s.AddMul, context); e != nil {
				return e
			}
			if e = m.Swap(a1, t); e != nil {
				return e
			}
			if e = m.Swap(b1, r); e != nil {
				return e
			}
			if e = m.gcdSourceGreaterEqualAt(a1, b1, 813, "ge(a1, b1)"); e != nil {
				return e
			}
		}
	}
}
