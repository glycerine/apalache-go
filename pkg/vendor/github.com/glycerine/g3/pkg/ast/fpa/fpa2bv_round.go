package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:3744-3791.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) roundingModeIs(bits ast.ASTHandle, code uint32) (ast.ASTHandle, error) {
	value, err := c.manager.BitVectorNumeralUnsigned(code, 3)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolEq(bits, value)
}

func (c *Converter) roundingDecisionBits(rm, sign, last, round, sticky ast.ASTHandle) (ast.ASTHandle, error) {
	for _, field := range []struct {
		value ast.ASTHandle
		width uint32
	}{{rm, 3}, {sign, 1}, {last, 1}, {round, 1}, {sticky, 1}} {
		width, err := c.manager.BitVectorSize(field.value)
		if err != nil || width != field.width {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	lastOrSticky, err := c.manager.MkBitVectorOr([]ast.ASTHandle{last, sticky})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	roundOrSticky, err := c.manager.MkBitVectorOr([]ast.ASTHandle{round, sticky})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	notRound, err := c.manager.MkBitVectorNot(round)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	notLastOrSticky, err := c.manager.MkBitVectorNot(lastOrSticky)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	notRoundOrSticky, err := c.manager.MkBitVectorNot(roundOrSticky)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	notSign, err := c.manager.MkBitVectorNot(sign)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	evenRefusal, err := c.manager.MkBitVectorOr([]ast.ASTHandle{notRound, notLastOrSticky})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	incrementEven, err := c.manager.MkBitVectorNot(evenRefusal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveRefusal, err := c.manager.MkBitVectorOr([]ast.ASTHandle{sign, notRoundOrSticky})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	incrementPositive, err := c.manager.MkBitVectorNot(positiveRefusal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeRefusal, err := c.manager.MkBitVectorOr([]ast.ASTHandle{notSign, notRoundOrSticky})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	incrementNegative, err := c.manager.MkBitVectorNot(negativeRefusal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := c.zero(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	isNegative, err := c.roundingModeIs(rm, 3)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := c.boolITE(isNegative, incrementNegative, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	isPositive, err := c.roundingModeIs(rm, 2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = c.boolITE(isPositive, incrementPositive, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	isAway, err := c.roundingModeIs(rm, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = c.boolITE(isAway, round, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	isEven, err := c.roundingModeIs(rm, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolITE(isEven, incrementEven, result)
}

// RoundingDecision returns the source rounder's one-bit increment decision.
// The rounding mode is the FPA BV2RM wrapper used by converter expressions.
func (c *Converter) RoundingDecision(roundingMode, sign, last, round, sticky ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.roundingDecisionBits(rm, sign, last, round, sticky)
}

type roundBuilder struct {
	c   *Converter
	err error
}

func (b *roundBuilder) call(f func() (ast.ASTHandle, error)) ast.ASTHandle {
	if b.err != nil {
		return ast.ASTHandle{}
	}
	var value ast.ASTHandle
	value, b.err = f()
	return value
}
func (b *roundBuilder) zero(n uint32) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.zero(n) })
}
func (b *roundBuilder) one(n uint32) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.one(n) })
}
func (b *roundBuilder) ones(n uint32) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.BitVectorNumeralAllOnes(n) })
}
func (b *roundBuilder) number(v, n uint32) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.BitVectorNumeralUnsigned(v, n) })
}
func (b *roundBuilder) extract(hi, lo uint32, v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorExtract(hi, lo, v) })
}
func (b *roundBuilder) zeroExtend(n uint32, v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorZeroExtend(n, v) })
}
func (b *roundBuilder) signExtend(n uint32, v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorSignExtend(n, v) })
}
func (b *roundBuilder) concat(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorConcatPair(a, c) })
}
func (b *roundBuilder) add(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorAdd(a, c) })
}
func (b *roundBuilder) sub(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorSub(a, c) })
}
func (b *roundBuilder) neg(v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorNeg(v) })
}
func (b *roundBuilder) bitOr(values ...ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorOr(values) })
}
func (b *roundBuilder) bitAnd(values ...ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorAnd(values) })
}
func (b *roundBuilder) bitXor(values ...ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorXor(values) })
}
func (b *roundBuilder) mul(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorMul(a, c) })
}
func (b *roundBuilder) udiv(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorUDivInternal(a, c) })
}
func (b *roundBuilder) bitNot(v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorNot(v) })
}
func (b *roundBuilder) ule(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorULE(a, c) })
}
func (b *roundBuilder) sle(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorSLE(a, c) })
}
func (b *roundBuilder) shl(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorSHL(a, c) })
}
func (b *roundBuilder) lshr(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorLSHR(a, c) })
}
func (b *roundBuilder) ashr(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorASHR(a, c) })
}
func (b *roundBuilder) urem(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorURem(a, c) })
}
func (b *roundBuilder) ult(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.unsignedLT(a, c) })
}
func (b *roundBuilder) reductionOr(v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorReductionOr(v) })
}
func (b *roundBuilder) reductionAnd(v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkBitVectorReductionAnd(v) })
}
func (b *roundBuilder) equal(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.boolEq(a, c) })
}
func (b *roundBuilder) and(values ...ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.boolAnd(values...) })
}
func (b *roundBuilder) or(values ...ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.boolOr(values...) })
}
func (b *roundBuilder) xor(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.boolXor(a, c) })
}
func (b *roundBuilder) not(v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.boolNot(v) })
}
func (b *roundBuilder) ite(condition, yes, no ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.boolITE(condition, yes, no) })
}
func (b *roundBuilder) fpITE(condition, yes, no ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.MkITE(condition, yes, no) })
}
func (b *roundBuilder) arithAdd(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkArithmeticAdd(a, c) })
}
func (b *roundBuilder) arithMul(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkArithmeticMul(a, c) })
}
func (b *roundBuilder) arithDiv(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkArithmeticDiv(a, c) })
}
func (b *roundBuilder) arithNeg(v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkArithmeticNeg(v) })
}
func (b *roundBuilder) arithPower(a, c ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.manager.MkArithmeticPower(a, c) })
}
func (b *roundBuilder) leading(v ast.ASTHandle, n uint32) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.LeadingZeros(v, n) })
}
func (b *roundBuilder) bias(v ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.Bias(v) })
}
func (b *roundBuilder) mode(rm ast.ASTHandle, code uint32) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.roundingModeIs(rm, code) })
}
func (b *roundBuilder) decision(rm, sign, last, round, sticky ast.ASTHandle) ast.ASTHandle {
	return b.call(func() (ast.ASTHandle, error) { return b.c.roundingDecisionBits(rm, sign, last, round, sticky) })
}

func (c *Converter) roundBits(sort, rm, sign, significand, exponent ast.ASTHandle) (ast.ASTHandle, error) {
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating || ebits < 2 || sbits < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	widths := []struct {
		value ast.ASTHandle
		want  uint32
	}{{rm, 3}, {sign, 1}, {significand, sbits + 4}, {exponent, ebits + 2}}
	for _, field := range widths {
		width, e := c.manager.BitVectorSize(field.value)
		if e != nil || width != field.want {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	sigSize, expSize := sbits+4, ebits+2
	if 2*sigSize < expSize {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	b := roundBuilder{c: c}
	one1 := b.one(1)
	minExponent := b.zero(ebits)
	if ebits > 2 {
		minExponent = b.concat(b.one(1), b.number(2, ebits-1))
	}
	maxExponent := b.concat(b.zero(1), b.ones(ebits-1))

	high := b.extract(ebits+1, ebits+1, exponent)
	second := b.extract(ebits, ebits, exponent)
	third := b.extract(ebits-1, ebits-1, exponent)
	exponentTopThree := b.and(b.not(b.equal(high, one1)), b.or(b.equal(second, one1), b.equal(third, one1)))
	exponentAtMaximum := b.equal(b.zeroExtend(2, maxExponent), exponent)
	significandHigh := b.equal(b.extract(sbits+3, sbits+3, significand), one1)
	overflow1 := b.or(exponentTopThree, b.and(exponentAtMaximum, significandHigh))

	leadingZeros := b.leading(significand, expSize)
	tinyOffset := b.sub(b.add(exponent, b.one(expSize)), leadingZeros)
	tinyOffset = b.sub(tinyOffset, b.signExtend(2, minExponent))
	tiny := b.sle(tinyOffset, b.ones(expSize))
	beta := b.add(b.sub(exponent, leadingZeros), b.one(expSize))
	sigmaAdd := b.add(b.sub(exponent, b.signExtend(2, minExponent)), b.one(expSize))
	sigma := b.ite(tiny, sigmaAdd, leadingZeros)

	sigmaNegative := b.neg(sigma)
	sigmaCap := b.number(sbits+2, expSize)
	cappedNegative := b.ite(b.ule(sigmaNegative, sigmaCap), sigmaNegative, sigmaCap)
	sigmaBelowZero := b.sle(sigma, b.ones(expSize))
	extendedSignificand := b.concat(significand, b.zero(sigSize))
	rightShifted := b.lshr(extendedSignificand, b.zeroExtend(2*sigSize-expSize, cappedNegative))
	leftShifted := b.shl(extendedSignificand, b.zeroExtend(2*sigSize-expSize, sigma))
	shifted := b.ite(sigmaBelowZero, rightShifted, leftShifted)
	lowBit := 2*sigSize - sbits - 2
	significand = b.extract(2*sigSize-1, lowBit, shifted)
	sticky := b.reductionOr(b.extract(lowBit-1, 0, shifted))
	significand = b.bitOr(significand, b.zeroExtend(sbits+1, sticky))
	exponent = b.ite(tiny, b.zeroExtend(2, minExponent), beta)

	sticky = b.extract(0, 0, significand)
	round := b.extract(1, 1, significand)
	last := b.extract(2, 2, significand)
	significand = b.extract(sbits+1, 2, significand)
	increment := b.decision(rm, sign, last, round, sticky)
	significand = b.add(b.zeroExtend(1, significand), b.zeroExtend(sbits, increment))
	significandOverflow := b.equal(b.extract(sbits, sbits, significand), one1)
	significand = b.ite(significandOverflow, b.extract(sbits, 1, significand), b.extract(sbits-1, 0, significand))
	exponent = b.ite(significandOverflow, b.add(exponent, b.one(expSize)), exponent)

	biasedExponent := b.bias(b.extract(ebits-1, 0, exponent))
	preOverflow2 := b.equal(b.reductionAnd(biasedExponent), one1)
	overflow2 := b.and(significandOverflow, preOverflow2)
	allButTwo := b.zero(ebits)
	if ebits > 2 {
		allButTwo = b.concat(b.zero(2), b.ones(ebits-2))
	}
	biasedExponent = b.ite(overflow2, allButTwo, biasedExponent)
	overflow := b.or(overflow1, overflow2)

	topExponent := b.ones(ebits)
	bottomExponent := b.zero(ebits)
	isTowardZero := b.mode(rm, 4)
	isTowardNegative := b.mode(rm, 3)
	isTowardPositive := b.mode(rm, 2)
	zeroOrNegative := b.or(isTowardZero, isTowardNegative)
	zeroOrPositive := b.or(isTowardZero, isTowardPositive)
	signIsZero := b.equal(sign, b.zero(1))
	maximumSignificand := b.ones(sbits - 1)
	maximumExponent := b.concat(b.ones(ebits-1), b.zero(1))
	infiniteSignificand := b.zero(sbits - 1)
	negativeOverflowExponent := b.ite(zeroOrPositive, maximumExponent, topExponent)
	positiveOverflowExponent := b.ite(zeroOrNegative, maximumExponent, topExponent)
	overflowExponent := b.ite(signIsZero, positiveOverflowExponent, negativeOverflowExponent)
	denormal := b.equal(b.extract(sbits-1, sbits-1, significand), b.zero(1))
	exponent = b.ite(overflow, overflowExponent, b.ite(denormal, bottomExponent, biasedExponent))
	negativeOverflowSignificand := b.ite(zeroOrPositive, maximumSignificand, infiniteSignificand)
	positiveOverflowSignificand := b.ite(zeroOrNegative, maximumSignificand, infiniteSignificand)
	overflowSignificand := b.ite(signIsZero, positiveOverflowSignificand, negativeOverflowSignificand)
	significand = b.ite(overflow, overflowSignificand, b.extract(sbits-2, 0, significand))
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	return c.MkFP(sign, exponent, significand)
}

// Round normalizes and rounds the converter's extended significand/exponent
// tuple into the requested floating-point format.
func (c *Converter) Round(sort, roundingMode, sign, significand, exponent ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.roundBits(sort, rm, sign, significand, exponent)
}
