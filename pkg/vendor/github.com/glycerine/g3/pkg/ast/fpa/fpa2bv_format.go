package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:2336-2502.

import "github.com/glycerine/g3/pkg/ast"

// ToFPFloat converts an encoded floating-point value to another floating-point
// format using the supplied rounding mode.
func (c *Converter) ToFPFloat(targetSort, roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	toEbits, toSbits, floating, err := c.manager.FloatingPointSortShape(targetSort)
	if err != nil || !floating || toEbits < 2 || toSbits < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	sourceSort, err := c.manager.GetSort(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fromEbits, fromSbits, floating, err := c.manager.FloatingPointSortShape(sourceSort)
	if err != nil || !floating || fromEbits < 2 || fromSbits < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if fromEbits == toEbits && fromSbits == toSbits {
		return value, nil
	}

	nan, err := c.MkSpecial(targetSort, NaN)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveZero, err := c.MkSpecial(targetSort, PositiveZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeZero, err := c.MkSpecial(targetSort, NegativeZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveInfinity, err := c.MkSpecial(targetSort, PositiveInfinity)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeInfinity, err := c.MkSpecial(targetSort, NegativeInfinity)
	if err != nil {
		return ast.ASTHandle{}, err
	}

	unpacked, err := c.Unpack(value, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b := roundBuilder{c: c}
	resSign := unpacked.Sign
	resSig := unpacked.Significand
	if fromSbits < toSbits+3 {
		resSig = b.concat(resSig, b.zero(toSbits+3-fromSbits))
	} else if fromSbits > toSbits+3 {
		high := b.extract(fromSbits-1, fromSbits-toSbits-2, resSig)
		low := b.extract(fromSbits-toSbits-3, 0, resSig)
		resSig = b.concat(high, b.reductionOr(low))
	}
	resSig = b.zeroExtend(1, resSig)

	var resExp ast.ASTHandle
	if fromEbits < toEbits+2 {
		extra := toEbits - fromEbits + 2
		resExp = b.sub(b.signExtend(extra, unpacked.Exponent), b.zeroExtend(extra, unpacked.LeadingZeros))
	} else if fromEbits > toEbits+2 {
		difference := fromEbits - (toEbits + 2)
		expMinusLeading := b.sub(b.signExtend(2, unpacked.Exponent), b.signExtend(2, unpacked.LeadingZeros))
		inRange := b.extract(toEbits+1, 0, expMinusLeading)

		// Largest and smallest signed exponents that the target rounder can
		// consume. These bit patterns mirror the source powers2 constants.
		maxExp := b.concat(b.zero(1), b.ones(toEbits))
		maxExp = b.concat(maxExp, b.zero(1))
		minExp := b.add(b.concat(b.one(1), b.zero(toEbits+1)), b.number(2, toEbits+2))
		firstOverflow := b.concat(b.zero(difference+3), b.ones(toEbits+1))
		firstUnderflow := b.concat(b.neg(b.one(difference+3)), b.one(toEbits+1))
		overflow := b.sle(firstOverflow, expMinusLeading)
		underflow := b.sle(expMinusLeading, firstUnderflow)
		resExp = b.ite(overflow, maxExp, inRange)
		resExp = b.ite(underflow, minExp, resExp)
	} else {
		resExp = b.sub(unpacked.Exponent, unpacked.LeadingZeros)
	}
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	rounded, err := c.roundBits(targetSort, rm, resSign, resSig, resExp)
	if err != nil {
		return ast.ASTHandle{}, err
	}

	isNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(value) })
	isZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(value) })
	isInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(value) })
	isNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(value) })
	isPositive := b.not(isNegative)
	isPositiveZero := b.and(isZero, isPositive)
	isNegativeZero := b.and(isZero, isNegative)
	isPositiveInfinity := b.and(isInfinite, isPositive)
	isNegativeInfinity := b.and(isInfinite, isNegative)
	result := rounded
	result = b.fpITE(isNegativeInfinity, negativeInfinity, result)
	result = b.fpITE(isPositiveInfinity, positiveInfinity, result)
	result = b.fpITE(isNegativeZero, negativeZero, result)
	result = b.fpITE(isPositiveZero, positiveZero, result)
	result = b.fpITE(isNaN, nan, result)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	return result, nil
}
