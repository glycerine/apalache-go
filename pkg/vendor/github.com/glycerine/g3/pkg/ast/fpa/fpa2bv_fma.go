package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:1298-1691.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) fmaBits(sort, rm, x, y, z ast.ASTHandle) (ast.ASTHandle, error) {
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating || 3*sbits+3 < ebits+2 || 2*sbits+3 < ebits {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	specials := make([]ast.ASTHandle, 5)
	for i, kind := range []SpecialValue{NaN, NegativeZero, PositiveZero, NegativeInfinity, PositiveInfinity} {
		specials[i], err = c.MkSpecial(sort, kind)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	nan, negativeZero, positiveZero, negativeInfinity, positiveInfinity := specials[0], specials[1], specials[2], specials[3], specials[4]
	b := roundBuilder{c: c}
	xNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(x) })
	yNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(y) })
	zNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(z) })
	xZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(x) })
	yZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(y) })
	zZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(z) })
	xPositive := b.call(func() (ast.ASTHandle, error) { return c.isPositiveSign(x) })
	yPositive := b.call(func() (ast.ASTHandle, error) { return c.isPositiveSign(y) })
	xNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(x) })
	yNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(y) })
	zNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(z) })
	xInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(x) })
	yInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(y) })
	zInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(z) })
	towardNegative := b.mode(rm, 3)
	xPositiveInfinity := b.and(xInfinite, xPositive)
	yPositiveInfinity := b.and(yInfinite, yPositive)
	xNegativeInfinity := b.and(xInfinite, xNegative)
	yNegativeInfinity := b.and(yInfinite, yNegative)
	productOpposesAddend := b.and(zInfinite, b.xor(b.xor(xNegative, yNegative), zNegative))
	condition1 := b.or(xNaN, yNaN, zNaN)
	value2 := b.fpITE(b.or(yZero, productOpposesAddend), nan, b.fpITE(yPositive, positiveInfinity, negativeInfinity))
	value3 := b.fpITE(b.or(xZero, productOpposesAddend), nan, b.fpITE(xPositive, positiveInfinity, negativeInfinity))
	value4 := b.fpITE(b.or(yZero, productOpposesAddend), nan, b.fpITE(yPositive, negativeInfinity, positiveInfinity))
	value5 := b.fpITE(b.or(xZero, productOpposesAddend), nan, b.fpITE(xPositive, negativeInfinity, positiveInfinity))
	condition7 := b.or(xZero, yZero)
	productSign := b.xor(xNegative, yNegative)
	zeroCancellation := b.and(zZero, b.xor(productSign, zNegative))
	zeroByMode := b.fpITE(towardNegative, negativeZero, positiveZero)
	value7 := b.fpITE(zeroCancellation, zeroByMode, z)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}

	a, err := c.Unpack(x, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := c.Unpack(y, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	e, err := c.Unpack(z, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b = roundBuilder{c: c}
	one1, zero1 := b.one(1), b.zero(1)
	aLeading := b.zeroExtend(2, a.LeadingZeros)
	dLeading := b.zeroExtend(2, d.LeadingZeros)
	eLeading := b.zeroExtend(2, e.LeadingZeros)
	aSig := b.zeroExtend(sbits, a.Significand)
	dSig := b.zeroExtend(sbits, d.Significand)
	aExp := b.signExtend(2, a.Exponent)
	dExp := b.signExtend(2, d.Exponent)
	eExp := b.signExtend(2, e.Exponent)
	mulSign := b.bitXor(a.Sign, d.Sign)
	mulExp := b.add(b.sub(aExp, aLeading), b.sub(dExp, dLeading))
	mulSig := b.mul(aSig, dSig)
	addendSig := b.zeroExtend(1, b.concat(e.Significand, b.zero(sbits+2)))
	eExp = b.sub(eExp, eLeading)
	mulSig = b.concat(mulSig, b.zero(3))
	swap := b.sle(mulExp, eExp)
	highSign := b.ite(swap, e.Sign, mulSign)
	highSig := b.ite(swap, addendSig, mulSig)
	highExp := b.ite(swap, eExp, mulExp)
	lowSign := b.ite(swap, mulSign, e.Sign)
	lowSig := b.ite(swap, mulSig, addendSig)
	lowExp := b.ite(swap, mulExp, eExp)
	exponentDelta := b.sub(highExp, lowExp)
	cap := b.number(2*sbits+3, ebits+2)
	exponentDelta = b.ite(b.ule(cap, exponentDelta), cap, exponentDelta)
	wideLow := b.concat(lowSig, b.zero(sbits))
	shiftedWide := b.lshr(wideLow, b.zeroExtend(3*sbits+3-(ebits+2), exponentDelta))
	shiftedLow := b.extract(3*sbits+2, sbits, shiftedWide)
	alignmentSticky := b.reductionOr(b.extract(sbits-1, 0, shiftedWide))
	highSig = b.zeroExtend(2, highSig)
	shiftedLow = b.zeroExtend(2, shiftedLow)
	sameSign := b.equal(highSign, lowSign)
	stickyWide := b.zeroExtend(2*sbits+4, alignmentSticky)
	plus := b.add(highSig, shiftedLow)
	plus = b.ite(b.equal(b.extract(0, 0, plus), zero1), b.add(plus, stickyWide), plus)
	minus := b.sub(highSig, shiftedLow)
	minus = b.ite(b.equal(b.extract(0, 0, minus), zero1), b.sub(minus, stickyWide), minus)
	sum := b.ite(sameSign, plus, minus)
	sumSign := b.extract(2*sbits+4, 2*sbits+4, sum)
	absSum := b.ite(b.equal(sumSign, one1), b.neg(sum), sum)
	notHighSign := b.bitNot(highSign)
	notLowSign := b.bitNot(lowSign)
	notSumSign := b.bitNot(sumSign)
	resultSign := b.bitOr(
		b.bitAnd(notHighSign, lowSign, sumSign),
		b.bitAnd(highSign, notLowSign, notSumSign),
		b.bitAnd(highSign, lowSign),
	)
	absSum = b.ite(b.equal(one1, b.extract(2*sbits+4, 2*sbits+4, absSum)), b.neg(absSum), absSum)
	extra := b.extract(2*sbits+4, 2*sbits+3, absSum)
	extraZero := b.equal(extra, b.zero(2))
	resultExp := b.ite(extraZero, highExp, b.add(highExp, b.one(ebits+2)))
	minExponent := b.zero(ebits)
	if ebits > 2 {
		minExponent = b.concat(one1, b.number(2, ebits-1))
	}
	minExponent = b.signExtend(2, minExponent)
	significandLeading := b.sub(b.leading(absSum, ebits+2), b.number(2, ebits+2))
	maximumExponentDelta := b.sub(resultExp, minExponent)
	cappedLeading := b.ite(b.sle(significandLeading, maximumExponentDelta), significandLeading, maximumExponentDelta)
	renormalize := b.ite(b.sle(b.zero(ebits+2), cappedLeading), cappedLeading, b.zero(ebits+2))
	resultExp = b.sub(resultExp, renormalize)
	absSum = b.shl(absSum, b.zeroExtend(2*sbits+3-ebits, renormalize))
	tooShort := uint32(0)
	if sbits < 5 {
		tooShort = 7 - sbits
		absSum = b.concat(absSum, b.zero(tooShort))
	}
	sticky1 := b.extract(sbits+tooShort-2, 0, absSum)
	high1 := b.extract(2*sbits+tooShort+4, sbits-1+tooShort, absSum)
	high1 = b.bitOr(high1, b.zeroExtend(sbits+5, b.reductionOr(sticky1)))
	resultSig1 := b.extract(sbits+3, 0, high1)
	sticky2 := b.extract(sbits+tooShort-1, 0, absSum)
	high2 := b.extract(2*sbits+tooShort+4, sbits+tooShort, absSum)
	_ = sticky2 // Source computes this slice but reduces sticky1 in this branch.
	high2 = b.zeroExtend(1, b.bitOr(high2, b.zeroExtend(sbits+4, b.reductionOr(sticky1))))
	resultSig2 := b.extract(sbits+3, 0, high2)
	resultSig := b.ite(extraZero, resultSig1, resultSig2)
	zeroSignificand := b.equal(resultSig, b.zero(sbits+4))
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	zeroCase, err := c.MkITE(towardNegative, negativeZero, positiveZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rounded, err := c.roundBits(sort, rm, resultSign, resultSig, resultExp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := c.MkITE(zeroSignificand, zeroCase, rounded)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, choice := range []struct {
		condition ast.ASTHandle
		value     ast.ASTHandle
	}{
		{condition7, value7},
		{zInfinite, z},
		{yNegativeInfinity, value5},
		{xNegativeInfinity, value4},
		{yPositiveInfinity, value3},
		{xPositiveInfinity, value2},
		{condition1, nan},
	} {
		result, err = c.MkITE(choice.condition, choice.value, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

// FMA converts the fused x*y+z operation with a single final rounding.
func (c *Converter) FMA(roundingMode, x, y, z ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.manager.GetSort(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, value := range []ast.ASTHandle{y, z} {
		valueSort, e := c.manager.GetSort(value)
		if e != nil || valueSort != sort {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.fmaBits(sort, rm, x, y, z)
}
