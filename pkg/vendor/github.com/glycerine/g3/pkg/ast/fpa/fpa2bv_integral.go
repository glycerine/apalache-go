package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:1842-2063.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) signedOne(sort, sign ast.ASTHandle) (ast.ASTHandle, error) {
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	zero, err := c.zero(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ones, err := c.manager.BitVectorNumeralAllOnes(ebits - 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := c.manager.MkBitVectorConcatPair(zero, ones)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	significand, err := c.zero(sbits - 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.MkFP(sign, exponent, significand)
}

func (c *Converter) roundToIntegralBits(sort, rm, value ast.ASTHandle) (ast.ASTHandle, error) {
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating || ebits > sbits {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	b := roundBuilder{c: c}
	nearestAway := b.mode(rm, 1)
	nearestEven := b.mode(rm, 0)
	towardPositive := b.mode(rm, 2)
	towardNegative := b.mode(rm, 3)
	towardZero := b.mode(rm, 4)
	nan := b.call(func() (ast.ASTHandle, error) { return c.MkSpecial(sort, NaN) })
	negativeZero := b.call(func() (ast.ASTHandle, error) { return c.MkSpecial(sort, NegativeZero) })
	positiveZero := b.call(func() (ast.ASTHandle, error) { return c.MkSpecial(sort, PositiveZero) })
	valueZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(value) })
	valueNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(value) })
	valueNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(value) })
	valueInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(value) })
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	unpacked, err := c.Unpack(value, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b = roundBuilder{c: c}
	one1, zero1 := b.one(1), b.zero(1)
	signNegative := b.equal(unpacked.Sign, one1)
	signedZero := b.fpITE(signNegative, negativeZero, positiveZero)
	exponentNegative := b.equal(b.extract(ebits-1, ebits-1, unpacked.Exponent), one1)
	positiveOne := b.call(func() (ast.ASTHandle, error) { return c.signedOne(sort, zero1) })
	negativeOne := b.call(func() (ast.ASTHandle, error) { return c.signedOne(sort, one1) })
	signedOne := b.fpITE(signNegative, negativeOne, positiveOne)
	halfPattern := b.concat(one1, b.zero(sbits-1))
	minusOneExponent := b.neg(b.one(ebits))
	tie := b.and(b.equal(unpacked.Significand, halfPattern), b.equal(unpacked.Exponent, minusOneExponent))
	tieEven := b.and(tie, nearestEven)
	tieAway := b.and(tie, nearestAway)
	atMostNegativeTwo := b.sle(unpacked.Exponent, b.neg(b.number(2, ebits)))
	nearestResult := signedOne
	nearestResult = b.fpITE(atMostNegativeTwo, signedZero, nearestResult)
	nearestResult = b.fpITE(tieAway, signedOne, nearestResult)
	nearestResult = b.fpITE(tieEven, signedZero, nearestResult)
	positiveResult := b.fpITE(valueNegative, negativeZero, positiveOne)
	negativeResult := b.fpITE(valueNegative, negativeOne, positiveZero)
	smallResult := b.fpITE(towardPositive, positiveResult, nearestResult)
	smallResult = b.fpITE(towardNegative, negativeResult, smallResult)
	smallResult = b.fpITE(towardZero, signedZero, smallResult)
	exponentLarge := b.sle(b.number(sbits-1, ebits), unpacked.Exponent)

	shift := b.sub(b.number(sbits-1, sbits), b.zeroExtend(sbits-ebits, unpacked.Exponent))
	zeroSig := b.zero(sbits)
	shifted := b.lshr(b.concat(unpacked.Significand, zeroSig), b.concat(zeroSig, shift))
	integerPart := b.extract(2*sbits-1, sbits, shifted)
	remainder := b.extract(sbits-1, 0, shifted)
	incremented := b.add(integerPart, b.one(sbits))
	tiePattern := b.concat(one1, b.zero(sbits-1))
	exactTie := b.equal(remainder, tiePattern)
	lastOne := b.equal(b.extract(0, 0, integerPart), one1)
	tieIncrement := b.or(b.and(nearestEven, lastOne), nearestAway)
	nearestIncrement := b.ite(exactTie, tieIncrement, b.ule(tiePattern, remainder))
	nearestSig := b.ite(nearestIncrement, incremented, integerPart)
	remainderZero := b.equal(remainder, b.zero(sbits))
	positiveIncrement := b.and(b.not(remainderZero), b.equal(unpacked.Sign, zero1))
	positiveSig := b.ite(positiveIncrement, incremented, integerPart)
	negativeIncrement := b.and(b.not(remainderZero), b.equal(unpacked.Sign, one1))
	negativeSig := b.ite(negativeIncrement, incremented, integerPart)
	resultSig := integerPart
	resultSig = b.ite(towardNegative, negativeSig, resultSig)
	resultSig = b.ite(towardPositive, positiveSig, resultSig)
	resultSig = b.ite(b.or(nearestEven, nearestAway), nearestSig, resultSig)
	resultSig = b.zeroExtend(1, b.concat(resultSig, b.zero(3)))
	var exponentShift ast.ASTHandle
	if ebits+2 <= sbits+1 {
		exponentShift = b.extract(ebits+1, 0, shift)
	} else {
		exponentShift = b.signExtend(ebits+2-sbits, shift)
	}
	resultExp := b.add(b.zeroExtend(2, unpacked.Exponent), exponentShift)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	rounded, err := c.roundBits(sort, rm, unpacked.Sign, resultSig, resultExp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b = roundBuilder{c: c}
	result := b.fpITE(exponentLarge, value, rounded)
	result = b.fpITE(exponentNegative, smallResult, result)
	result = b.fpITE(valueZero, value, result)
	result = b.fpITE(valueInfinite, value, result)
	result = b.fpITE(valueNaN, nan, result)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	return result, nil
}

// RoundToIntegral converts the IEEE round-to-integral operation to BV terms.
func (c *Converter) RoundToIntegral(roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.manager.GetSort(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.roundToIntegralBits(sort, rm, value)
}
