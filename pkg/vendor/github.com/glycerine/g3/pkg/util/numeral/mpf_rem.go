package numeral

// Source: src/util/mpf.cpp:1438-1485; src/util/mpf.h:135.

import umemory "github.com/glycerine/g3/pkg/util/memory"

func (m *ArbitraryFloatManager) Rem(x, y, out *ArbitraryFloat) (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.rem.active {
		return ErrNumericPending
	}
	for _, v := range [...]*ArbitraryFloat{x, y, out} {
		if e := m.valueGuard(v); e != nil {
			return e
		}
	}
	if e := mpfSourceAssertion(x.SBits() == y.SBits() && x.EBits() == y.EBits(), 1439, "x.sbits == y.sbits && x.ebits == y.ebits"); e != nil {
		return e
	}
	if e := m.traceRemObjects(x, y); e != nil {
		return e
	}
	defer func() {
		if cause == nil {
			cause = m.traceRemObjectResult(out)
		}
		if cause == nil {
			cause = m.traceFloatDebugValueChecked("rem", "REMAINDER = ", out, 1484)
		}
	}()
	for _, v := range [...]*ArbitraryFloat{x, y} {
		nan, e := m.IsNaN(v)
		if e != nil {
			return e
		}
		if nan {
			return m.MkNaN(x.EBits(), x.SBits(), out, nil)
		}
	}
	inf, e := m.IsInf(x)
	if e != nil {
		return e
	}
	if inf {
		return m.MkNaN(x.EBits(), x.SBits(), out, nil)
	}
	inf, e = m.IsInf(y)
	if e != nil {
		return e
	}
	if inf {
		return m.Copy(out, x)
	}
	zero, e := m.IsZero(y)
	if e != nil {
		return e
	}
	if zero {
		return m.MkNaN(x.EBits(), x.SBits(), out, nil)
	}
	zero, e = m.IsZero(x)
	if e != nil {
		return e
	}
	if zero {
		return m.Copy(out, x)
	}
	assertionsEnabled, e := mpfSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if assertionsEnabled {
		xRegular, e := m.IsRegular(x)
		if e != nil {
			return e
		}
		yRegular := false
		if xRegular {
			yRegular, e = m.IsRegular(y)
			if e != nil {
				return e
			}
		}
		if e := mpfSourceAssertion(xRegular && yRegular, 1455, "is_regular(x) && is_regular(y)"); e != nil {
			return e
		}
	}
	s := &m.rem
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatRemScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, aLive: true, bLive: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishRemSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	c, z := s.context(), m.integer
	if e := m.copyWithValueReceiptAndContext(&s.a, x, &s.copyA, c); e != nil {
		return e
	}
	if e := m.copyWithValueReceiptAndContext(&s.b, y, &s.copyB, c); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.a, true, c); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.b, true, c); e != nil {
		return e
	}
	b := int64(x.SBits())
	for {
		limit, e := roundSignedSub(s.b.exponent, 1)
		if e != nil {
			return e
		}
		d := int64(0)
		if s.a.exponent >= limit {
			d, e = roundSignedSub(s.a.exponent, s.b.exponent)
			if e != nil {
				return e
			}
			before := m.partialRemainder.invocation
			child := m.PartialRemainder(&s.a, &s.b, d, d >= b)
			if m.partialRemainder.invocation != before {
				s.partialInvocation = m.partialRemainder.invocation
				s.partialDone = !m.partialRemainder.active
			}
			if child != nil {
				return child
			}
		}
		if d < b {
			break
		}
		zero, e := m.IsZero(&s.a)
		if e != nil {
			return e
		}
		if zero {
			break
		}
	}
	if e := z.check(&s.a.significand); e != nil {
		return e
	}
	if e := z.mul2kCoreWithAllocationContext(&s.a.significand, 3, c); e != nil {
		return e
	}
	// Same-format SET remains the source COPY fast path. Its actual receipt
	// belongs to this invocation even when the following Round reuses a frame.
	before := m.formattedSet.invocation
	child := m.SetFormatWithReceipt(out, x.EBits(), x.SBits(), ArbitraryFloatTowardZero, &s.a, &s.formatReceipt)
	if m.formattedSet.invocation != before {
		s.formatSourceCause = m.formattedSet.sourceCause
	}
	if child != nil {
		return child
	}
	return m.roundWithParentContext(ArbitraryFloatNearestEven, out, &s.roundReceipt, c)
}
