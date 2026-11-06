package numeral

// Source: src/util/mpf.cpp:589-665; src/util/mpf.h:126.

import (
	"github.com/glycerine/g3/pkg/arena"
	"math"
)

func (m *ArbitraryFloatManager) Mul(mode ArbitraryFloatRoundingMode, x, y, out *ArbitraryFloat) (cause error) {
	for _, v := range [...]*ArbitraryFloat{x, y, out} {
		if e := m.valueGuard(v); e != nil {
			return e
		}
	}
	if m.mul.active {
		return ErrNumericPending
	}
	if e := m.traceMulEntry(mode, x, y); e != nil {
		return e
	}
	defer func() {
		if cause == nil {
			cause = m.traceMulResult(out)
		}
	}()
	if e := mpfSourceAssertion(x.SBits() == y.SBits() && x.EBits() == y.EBits(), 593, "x.sbits == y.sbits && x.ebits == y.ebits"); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("mul", "X = ", x, 595); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("mul", "Y = ", y, 596); e != nil {
		return e
	}
	for _, v := range [...]*ArbitraryFloat{x, y} {
		nan, e := m.IsNaN(v)
		if e != nil {
			return e
		}
		if nan {
			return m.MkNaN(x.EBits(), x.SBits(), out, nil)
		}
	}
	// Preserve the four ordered source infinity queries and their zero tests.
	for _, branch := range [...]struct {
		v, other *ArbitraryFloat
		negative bool
	}{{x, y, false}, {y, x, false}, {x, y, true}, {y, x, true}} {
		var inf bool
		var e error
		if branch.negative {
			inf, e = m.IsNegativeInf(branch.v)
		} else {
			inf, e = m.IsPositiveInf(branch.v)
		}
		if e != nil {
			return e
		}
		if inf {
			zero, e := m.IsZero(branch.other)
			if e != nil {
				return e
			}
			if zero {
				return m.MkNaN(x.EBits(), x.SBits(), out, nil)
			}
			return m.MkInf(x.EBits(), x.SBits(), branch.other.Sign() != branch.negative, out)
		}
	}
	zero, e := m.IsZero(x)
	if e != nil {
		return e
	}
	if !zero {
		zero, e = m.IsZero(y)
		if e != nil {
			return e
		}
	}
	if zero {
		return m.MkZero(x.EBits(), x.SBits(), x.Sign() != y.Sign(), out)
	}
	out.assignEbits(x.EBits())
	out.assignSbits(x.SBits())
	out.assignSign(x.Sign() != y.Sign())
	s := &m.mul
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatMulScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, aLive: true, bLive: true}
	// scoped_mpf(manager, ebits, sbits) sets format and exponent, without SET0.
	s.a.format = arbitraryFloatFormat(x.EBits(), x.SBits(), false)
	s.b.format = arbitraryFloatFormat(x.EBits(), x.SBits(), false)
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishMulSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	context := s.context()
	z := m.integer
	if e := m.copyWithValueReceiptAndContext(&s.a, x, &s.copyA, context); e != nil {
		return e
	}
	if e := m.copyWithValueReceiptAndContext(&s.b, y, &s.copyB, context); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.a, true, context); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.b, true, context); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("mul", "A = ", &s.a, 640); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("mul", "B = ", &s.b, 641); e != nil {
		return e
	}
	a, b := s.a.exponent, s.b.exponent
	if b > 0 && a > math.MaxInt64-b || b < 0 && a < math.MinInt64-b {
		return arena.ErrBounds
	}
	out.exponent = a + b
	if e := m.traceFloatDebugIntChecked("mul", "A' = ", &s.a.significand, 645); e != nil {
		return e
	}
	if e := m.traceFloatDebugIntChecked("mul", "B' = ", &s.b.significand, 646); e != nil {
		return e
	}
	if e := z.mulWithAllocationContext(&s.a.significand, &s.b.significand, &out.significand, context); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("mul", "PRODUCT = ", out, 650); e != nil {
		return e
	}
	s.stickyLive = true
	if out.SBits() >= 4 {
		p, e := s.power(out.SBits() - 4)
		if e != nil {
			return e
		}
		if e := z.machineDivRemWithAllocationContext(&out.significand, &p, &out.significand, &s.sticky, context); e != nil {
			return e
		}
	} else {
		if e := z.shiftWithAllocationContext(&out.significand, 4-out.SBits(), &out.significand, true, context); e != nil {
			return e
		}
	}
	empty, e := z.IsZero(&s.sticky)
	if e != nil {
		return e
	}
	if !empty {
		even, e := z.IsEven(&out.significand)
		if e != nil {
			return e
		}
		if even {
			one := NewInt(1)
			if e := z.addSubWithAllocationContext(&out.significand, &one, &out.significand, false, context); e != nil {
				return e
			}
		}
	}
	return m.roundWithParentContext(mode, out, &s.roundReceipt, context)
}
