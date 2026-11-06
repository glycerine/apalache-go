package numeral

// Source: src/util/mpf.cpp:1948-2098; src/util/mpf.h:31-37.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type ArbitraryFloatRoundingMode uint8

const (
	ArbitraryFloatNearestEven ArbitraryFloatRoundingMode = iota
	ArbitraryFloatNearestAway
	ArbitraryFloatTowardPositive
	ArbitraryFloatTowardNegative
	ArbitraryFloatTowardZero
)

func roundSignedSub(a, b int64) (int64, error) {
	if b > 0 && a < math.MinInt64+b || b < 0 && a > math.MaxInt64+b {
		return 0, arena.ErrBounds
	}
	return a - b, nil
}
func (m *ArbitraryFloatManager) roundPower(n uint32, minusOne bool) (Int, error) {
	saved := m.valueExecuting
	m.valueExecuting = false
	defer func() { m.valueExecuting = saved }()
	var w *arbitraryFloatPowerWrapper
	var e error
	if minusOne {
		w, e = m.powerTwoMinusOneWrapper(n, false)
	} else {
		w, e = m.powerTwoWrapper(n, false)
	}
	if e != nil {
		return Int{}, e
	}
	return w.Capture()
}
func (m *ArbitraryFloatManager) roundBound(ebits uint32, min bool) (int64, error) {
	saved := m.valueExecuting
	m.valueExecuting = false
	defer func() { m.valueExecuting = saved }()
	if min {
		return m.minExponent(ebits)
	}
	return m.maxExponent(ebits)
}
func (m *ArbitraryFloatManager) roundOverflow(mode ArbitraryFloatRoundingMode, x *ArbitraryFloat) error {
	sign := x.Sign()
	maximum := mode == ArbitraryFloatTowardZero || !sign && mode == ArbitraryFloatTowardNegative || sign && mode == ArbitraryFloatTowardPositive
	x.assignSbits(x.SBits())
	x.assignEbits(x.EBits())
	x.assignSign(sign)
	saved := m.valueExecuting
	m.valueExecuting = false
	top, e := m.topExponent(x.EBits())
	m.valueExecuting = saved
	if e != nil {
		return e
	}
	if !maximum {
		x.exponent = top
		return m.integer.SetInt32(&x.significand, 0)
	}
	exponent, e := roundSignedSub(top, 1)
	if e != nil {
		return e
	}
	x.exponent = exponent
	p, e := m.roundPower(x.SBits()-1, true)
	if e != nil {
		return e
	}
	return m.integer.copyFromOwnerWithAllocationContext(&x.significand, m.integer, &p, nil, integerAllocationInvocationContext{mpfRound: &m.RoundScope, mpfRoundInvocation: m.RoundScope.Invocation})
}

func (m *ArbitraryFloatManager) Round(mode ArbitraryFloatRoundingMode, x *ArbitraryFloat) (err error) {
	return m.RoundWithReceipt(mode, x, nil)
}
func (m *ArbitraryFloatManager) RoundWithReceipt(mode ArbitraryFloatRoundingMode, x *ArbitraryFloat, receipt *ArbitraryFloatRoundReceipt) error {
	return m.roundWithParentContext(mode, x, receipt, integerAllocationInvocationContext{})
}
func (m *ArbitraryFloatManager) roundWithParentContext(mode ArbitraryFloatRoundingMode, x *ArbitraryFloat, receipt *ArbitraryFloatRoundReceipt, parent integerAllocationInvocationContext) (err error) {
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if parent.mpfFormattedSet != nil && parent.formattedSetFrame(m.integer, true) == nil {
		return umemory.ErrRuntimeOwner
	}
	s := &m.RoundScope
	if s.active {
		return ErrNumericPending
	}
	if s.Invocation == ^uint64(0) {
		return ErrNumericPending
	}
	if e := m.traceRoundValue("RND: ", x, 1967, true, 1, 3, ""); e != nil {
		return e
	}
	invocation := s.Invocation + 1
	*s = ArbitraryFloatRoundScope{Manager: m, Invocation: invocation, active: true, executing: true, receipt: receipt, parentFormattedSet: parent}
	if parent.Caller != nil {
		if caller, ok := parent.Caller.Policy.(interface {
			enterArbitraryFloatRound(*ArbitraryFloatManager, *ArbitraryFloatRoundScope) error
		}); ok {
			if e := caller.enterArbitraryFloatRound(m, s); e != nil {
				s.executing = false
				return e
			}
		}
	}
	if rs := parent.rationalSetFrame(m.integer, true); rs != nil {
		rs.roundFrame = s
		rs.roundInvocation = invocation
	}
	if fs := parent.formattedSetFrame(m.integer, true); fs != nil {
		fs.roundFrame = s
		fs.roundInvocation = invocation
	}
	m.valueExecuting = true
	defer func() {
		if err == nil {
			err = m.traceRoundValue("ROUNDED = ", x, 2096, true, ^uint32(0), 0, " (hidden bit, unbiased exp).")
		}
		m.valueExecuting = false
		s.executing = false
		s.bodyCause = err
		err = errors.Join(err, m.finishRoundSource())
		s.recordRoundHistory()
	}()
	z := m.integer
	context := integerAllocationInvocationContext{mpfRound: s, mpfRoundInvocation: s.Invocation}
	one := NewInt(1)
	sb := x.SBits()
	eb := x.EBits()
	if modularSourceDebug {
		p, e := m.roundPower(sb+4, false)
		if e != nil {
			return e
		}
		enabled, e := mpfSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled {
			c, e := z.Compare(&x.significand, &p)
			if e != nil {
				return e
			}
			if e := mpfSourceAssertion(c < 0, 1972, "m_mpz_manager.lt(o.significand, p_m3)"); e != nil {
				return e
			}
		}
	}
	emax, e := m.roundBound(eb, false)
	if e != nil {
		return e
	}
	emin, e := m.roundBound(eb, true)
	if e != nil {
		return e
	}
	width, e := z.PrevPowerOfTwo(&x.significand)
	if e != nil {
		return e
	}
	width++
	lz := int64(sb + 4 - width) // source unsigned32 subtraction before signed assignment
	beta, e := roundSignedSub(x.exponent, lz)
	if e != nil {
		return e
	}
	if beta == math.MaxInt64 {
		return arena.ErrBounds
	}
	beta++
	s.sigmaConstructed = true
	if beta < emin {
		delta, e := roundSignedSub(x.exponent, emin)
		if e != nil {
			return e
		}
		if e = z.setInt64WithAllocationContext(&s.sigma, delta, context); e != nil {
			return e
		}
		x.exponent = emin
	} else {
		if e = z.setInt64WithAllocationContext(&s.sigma, lz-1, context); e != nil {
			return e
		}
		x.exponent = beta
	}
	s.capConstructed = true
	if e = z.setUint32WithAllocationContext(&s.cap, sb+2, context); e != nil {
		return e
	}
	if e = z.Neg(&s.cap); e != nil {
		return e
	}
	c, e := z.Compare(&s.sigma, &s.cap)
	if e != nil {
		return e
	}
	if c < 0 {
		if e = z.copyFromOwnerWithAllocationContext(&s.sigma, z, &s.cap, nil, context); e != nil {
			return e
		}
	}
	if e := m.traceRoundNormalization(emin, emax, beta, lz, beta < emin, &s.sigma, &s.cap); e != nil {
		return e
	}
	minusOne := NewInt(-1)
	c, e = z.Compare(&s.sigma, &minusOne)
	if e != nil {
		return e
	}
	sigma, e := z.Int64(&s.sigma)
	if e != nil {
		return e
	}
	m.traceRoundShiftDistance(sigma)
	if c <= 0 {
		s.stickyConstructed = true
		p, e := m.roundPower(uint32(-sigma), false)
		if e != nil {
			return e
		}
		if e = z.machineDivRemWithAllocationContext(&x.significand, &p, &x.significand, &s.sticky, context); e != nil {
			return e
		}
		zero, e := z.IsZero(&s.sticky)
		if e != nil {
			return e
		}
		if !zero {
			even, e := z.IsEven(&x.significand)
			if e != nil {
				return e
			}
			if even {
				if e = z.addSubWithAllocationContext(&x.significand, &one, &x.significand, false, context); e != nil {
					return e
				}
			}
		}
		if _, e = SourceNumeralDel(z, &s.sticky); e != nil {
			s.sourceCause = e
			return e
		}
		s.stickyConstructed = false
	} else {
		if e = z.mul2kCoreWithAllocationContext(&x.significand, uint32(sigma), context); e != nil {
			return e
		}
	}
	if e := m.traceRoundValue("Shifted: ", x, 2026, true, 1, 3, ""); e != nil {
		return e
	}
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
	even, e = z.IsEven(&x.significand)
	if e != nil {
		return e
	}
	last := !even
	m.traceRoundBits(x.Sign(), last, round, sticky)
	if e := m.traceRoundValue("before rounding decision: ", x, 2041, false, 0, 0, ""); e != nil {
		return e
	}
	inc := false
	switch mode {
	case ArbitraryFloatNearestEven:
		inc = round && (last || sticky)
	case ArbitraryFloatNearestAway:
		inc = round
	case ArbitraryFloatTowardPositive:
		inc = !x.Sign() && (round || sticky)
	case ArbitraryFloatTowardNegative:
		inc = x.Sign() && (round || sticky)
	case ArbitraryFloatTowardZero:
	default:
		if e := diagnostic.BorrowProcessDebugContext().UnreachableSource("src/util/mpf.cpp", 2052); e != nil {
			return e
		}
	}
	m.traceRoundFlag(2055, "Rounding increment -> significand +", inc)
	if inc {
		if e = z.addSubWithAllocationContext(&x.significand, &one, &x.significand, false, context); e != nil {
			return e
		}
	}
	if e := m.traceRoundValue("Rounded significand: ", x, 2059, false, 0, 0, ""); e != nil {
		return e
	}
	p, e := m.roundPower(sb, false)
	if e != nil {
		return e
	}
	c, e = z.Compare(&x.significand, &p)
	if e != nil {
		return e
	}
	if c >= 0 {
		if e = z.MachineDiv2kInPlace(&x.significand, 1); e != nil {
			return e
		}
		if x.exponent == math.MaxInt64 {
			return arena.ErrBounds
		}
		x.exponent++
	}
	sigOverflow := x.exponent > emax
	if e := m.traceRoundValue("Post-normalized: ", x, 2070, false, 0, 0, ""); e != nil {
		return e
	}
	m.traceRoundFlag(2071, "SIGovf = ", sigOverflow)
	hasMaxExponent := x.exponent > emax
	secondOverflow := sigOverflow && hasMaxExponent
	m.traceRoundFlag(2078, "OVF2 = ", secondOverflow)
	m.traceRoundFlag(2079, "o_has_max_exp = ", hasMaxExponent)
	if secondOverflow {
		return m.roundOverflow(mode, x)
	}
	p, e = m.roundPower(sb-1, false)
	if e != nil {
		return e
	}
	c, e = z.Compare(&x.significand, &p)
	if e != nil {
		return e
	}
	if c >= 0 {
		if e := m.traceRoundSignificandBranch(&x.significand, true); e != nil {
			return e
		}
		return z.addSubWithAllocationContext(&x.significand, &p, &x.significand, true, context)
	}
	if e := m.traceRoundSignificandBranch(&x.significand, false); e != nil {
		return e
	}
	saved := m.valueExecuting
	m.valueExecuting = false
	bottom, e := m.bottomExponent(eb)
	m.valueExecuting = saved
	if e != nil {
		return e
	}
	x.exponent = bottom
	return nil
}
