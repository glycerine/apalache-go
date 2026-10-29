package numeral

// Source: src/util/mpz.h:allocate; src/util/mpz.cpp scratch/publication callers.

// Explicit passed invocation identity. It carries no allocation snapshot and
// creates no source numeral or reservation.
type integerAllocationInvocationContext struct {
	rationalPower    *RationalPowerInvocation
	Caller           *NumericInvocationHooks
	callerGeneration uint64

	rationalService           *RationalServiceInvocation
	rationalServiceInvocation uint64

	mpfRationalSet            *arbitraryFloatRationalSetScope
	mpfRationalSetInvocation  uint64
	mpfFormattedSet           *arbitraryFloatFormattedSetScope
	mpfFormattedSetInvocation uint64

	mpfInput                *arbitraryFloatInputBinding
	mpfInputInvocation      uint64
	mpfConversion           *arbitraryFloatRationalConversionScope
	mpfConversionInvocation uint64
	mpfConversionOutput     bool
	mpfFormat               *ArbitraryFloatFormatInvocation
	mpfFormatInvocation     uint64
	formatDecimal           *rationalDecimalScope
	mpfPower                *arbitraryFloatPowerWrapper
	mpfPowerInvocation      uint64

	mpfRound               *ArbitraryFloatRoundScope
	mpfRoundInvocation     uint64
	mpfRoundSqrt           *ArbitraryFloatRoundScope
	mpfRoundSqrtInvocation uint64

	BoolDivides           *integerDividesNumeralInvocationReceipt
	BoolDividesInvocation uint64

	raw           *integerRawInvocationReceipt
	rawInvocation uint64
	pn            *integerPNormalizeScope
	pnInvocation  uint64

	addMul           *integerAddMulInvocationReceipt
	addMulInvocation uint64

	perfectSquare           *integerPerfectSquareInvocationReceipt
	perfectSquareInvocation uint64
}

func (c integerAllocationInvocationContext) withAddMulInvocation(r *integerAddMulInvocationReceipt) integerAllocationInvocationContext {
	c.addMul = r
	c.addMulInvocation = 0
	if r != nil {
		c.addMulInvocation = r.invocation
	}
	return c
}
func (c integerAllocationInvocationContext) withRawInvocation(r *integerRawInvocationReceipt) integerAllocationInvocationContext {
	c.raw = r
	c.rawInvocation = 0
	if r != nil {
		c.rawInvocation = r.Invocation
	}
	return c
}
func (c integerAllocationInvocationContext) rawReceipt(m *IntegerManager) *integerRawInvocationReceipt {
	if c.raw != nil && c.rawInvocation != 0 && c.rawInvocation == c.raw.Invocation && m.RawAllocationRoots(c.raw) != nil {
		return c.raw
	}
	return nil
}

func pnAllocationContext(pn *integerPNormalizeScope) integerAllocationInvocationContext {
	c := integerAllocationInvocationContext{pn: pn}
	if pn != nil {
		c.pnInvocation = pn.invocation
	}
	return c
}
func (c integerAllocationInvocationContext) localRoots(m *IntegerManager) *NumericAllocationRoots {

	if c.rationalService != nil && c.rationalServiceFrame(m) == nil {
		return nil
	}

	if c.mpfRationalSet != nil && c.rationalSetFrame(m, true) == nil {
		return nil
	}

	if c.mpfFormattedSet != nil {
		if s := c.formattedSetFrame(m, true); s != nil && s.history != nil {
			return &s.history.Numeric
		}
		return nil
	}
	if c.mpfRound != nil || c.mpfRoundSqrt != nil {
		if s := c.roundFrame(m, true); s != nil {
			return &s.Numeric
		}
		return nil
	}
	if c.mpfInput != nil {
		return c.inputRoots(m)
	}
	if c.mpfConversion != nil && c.conversionFrame(m, true) == nil {
		return nil
	}
	if c.mpfFormat != nil && c.formatFrame(m, true) == nil {
		return nil
	}

	if c.perfectSquare != nil && m.perfectSquareContextRoots(c) == nil {
		return nil
	}

	if c.mpfPower != nil {
		return c.mpfPowerRoots(m)
	}

	if c.BoolDivides != nil {
		return m.dividesNumeralContextRoots(c)
	}

	if c.raw != nil {
		if r := c.rawReceipt(m); r != nil {
			return m.RawAllocationRoots(r)
		}
		return nil
	}
	if c.rationalService != nil {
		return &c.rationalService.numeric
	}
	if c.mpfRationalSet != nil {
		if h := c.mpfRationalSet.history; h != nil {
			return &h.Numeric
		}
		return nil
	}
	if c.mpfConversion != nil {
		if c.mpfConversionOutput {
			return &c.mpfConversion.outputAllocation.numeric
		}
		return &c.mpfConversion.numeric
	}
	if c.mpfFormat != nil {
		if c.formatDecimal != nil {
			return &c.formatDecimal.Numeric
		}
		return &c.mpfFormat.Numeric
	}
	if s := c.pn; s != nil && s.owner != nil && s.owner.manager == m && s == &s.owner.Normalization && s.active && s.executing && c.pnInvocation != 0 && c.pnInvocation == s.invocation {
		return &s.Roots
	}
	if c.pn != nil {
		return nil
	}

	if c.perfectSquare != nil {
		return m.perfectSquareContextRoots(c)
	}

	if c.addMul != nil && c.addMulInvocation != 0 && c.addMulInvocation == c.addMul.invocation && m.MatchesAddMulInvocation(c.addMul) && c.addMul.Frame.executing {
		s := c.addMul.Frame
		if s.parentContext != (integerAllocationInvocationContext{}) && s.parentContext.roots(m) == nil {
			return nil
		}

		return &s.Numeric
	}
	return nil
}
