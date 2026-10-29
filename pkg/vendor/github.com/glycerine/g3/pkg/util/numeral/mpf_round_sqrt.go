package numeral

// Source: src/util/mpf.cpp:2100-2131; src/util/mpf.h:229.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// RoundSqrt follows the source's specialized guard-bit switch. The caller owns
// the significand and exponent; this helper declares no lexical NUM or RAW.
func (m *ArbitraryFloatManager) RoundSqrt(mode ArbitraryFloatRoundingMode, x *ArbitraryFloat) error {
	return m.RoundSqrtWithReceipt(mode, x, nil)
}
func (m *ArbitraryFloatManager) traceRoundSqrtResult(x *ArbitraryFloat) error {
	saved := m.valueExecuting
	m.valueExecuting = false
	defer func() { m.valueExecuting = saved }()
	return m.traceFloatDebugValueChecked("round_sqrt", "ROUNDED = ", x, 2130)
}

func (m *ArbitraryFloatManager) traceRoundSqrtDecision(last, round, sticky, inc bool) {
	tags := m.floatTraceTags()
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	_ = tags.EmitConditional("mpf_dbg", "round_sqrt", "src/util/mpf.cpp", 2123,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{"last=", traceBoolDigit(last), " round=", traceBoolDigit(round), " sticky=", traceBoolDigit(sticky), " --> inc=", traceBoolDigit(inc), "\n"} {
				if e := writeRoundTracePart(w, part); e != nil {
					return e
				}
			}
			return nil
		})
}
func (m *ArbitraryFloatManager) RoundSqrtWithReceipt(mode ArbitraryFloatRoundingMode, x *ArbitraryFloat, r *ArbitraryFloatRoundReceipt) (err error) {
	if e := m.valueGuard(x); e != nil {
		return e
	}
	s := &m.RoundSqrtScope
	if s.active || s.Invocation == ^uint64(0) {
		return ErrNumericPending
	}
	if e := m.traceFloatDebugValueChecked("round_sqrt", "RND-SQRT: ", x, 2101); e != nil {
		return e
	}
	invocation := s.Invocation + 1
	*s = ArbitraryFloatRoundScope{Manager: m, Invocation: invocation, active: true, executing: true, receipt: r}
	m.valueExecuting = true
	defer func() {
		if err == nil {
			err = m.traceRoundSqrtResult(x)
		}
		m.valueExecuting = false
		s.executing = false
		s.bodyCause = err
		err = errors.Join(err, m.finishRoundSqrtSource())
		s.recordRoundHistory()
	}()
	z := m.integer
	c := integerAllocationInvocationContext{mpfRoundSqrt: s, mpfRoundSqrtInvocation: invocation}
	even, e := z.IsEven(&x.significand)
	if e != nil {
		return e
	}
	sticky := !even
	if e = z.MachineDiv2kInPlace(&x.significand, 1); e != nil {
		return e
	}
	if !sticky {
		even, e = z.IsEven(&x.significand)
		if e != nil {
			return e
		}
		sticky = !even
	}
	if e = z.MachineDiv2kInPlace(&x.significand, 1); e != nil {
		return e
	}
	even, e = z.IsEven(&x.significand)
	if e != nil {
		return e
	}
	round := !even
	if e = z.MachineDiv2kInPlace(&x.significand, 1); e != nil {
		return e
	}
	// Source computes last even though the specialized switch does not use it.
	even, e = z.IsEven(&x.significand)
	if e != nil {
		return e
	}
	last := !even
	inc := false
	switch mode {
	case ArbitraryFloatNearestEven, ArbitraryFloatNearestAway:
		inc = round && sticky
	case ArbitraryFloatTowardNegative, ArbitraryFloatTowardZero:
	case ArbitraryFloatTowardPositive:
		inc = round || sticky
	default:
		if e := diagnostic.BorrowProcessDebugContext().UnreachableSource("src/util/mpf.cpp", 2120); e != nil {
			return e
		}
	}
	m.traceRoundSqrtDecision(last, round, sticky, inc)
	if inc {
		one := NewInt(1)
		if e = z.addSubWithAllocationContext(&x.significand, &one, &x.significand, false, c); e != nil {
			return e
		}
	}
	p, e := m.roundPower(x.SBits()-1, false)
	if e != nil {
		return e
	}
	return z.addSubWithAllocationContext(&x.significand, &p, &x.significand, true, c)
}
