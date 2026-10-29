package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:670-818.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) mulBits(sort, rm, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	_, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
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
	leftNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(left) })
	rightNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(right) })
	leftZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(left) })
	rightZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(right) })
	leftPositive := b.call(func() (ast.ASTHandle, error) { return c.isPositiveSign(left) })
	rightPositive := b.call(func() (ast.ASTHandle, error) { return c.isPositiveSign(right) })
	leftInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(left) })
	rightInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(right) })
	leftPositiveInfinity := b.and(leftInfinite, leftPositive)
	rightPositiveInfinity := b.and(rightInfinite, rightPositive)
	leftNegativeInfinity := b.and(leftInfinite, b.not(leftPositive))
	rightNegativeInfinity := b.and(rightInfinite, b.not(rightPositive))
	condition1 := b.or(leftNaN, rightNaN)
	value2 := b.fpITE(rightZero, nan, b.fpITE(rightPositive, positiveInfinity, negativeInfinity))
	value3 := b.fpITE(leftZero, nan, b.fpITE(leftPositive, positiveInfinity, negativeInfinity))
	value4 := b.fpITE(rightZero, nan, b.fpITE(rightPositive, negativeInfinity, positiveInfinity))
	value5 := b.fpITE(leftZero, nan, b.fpITE(leftPositive, negativeInfinity, positiveInfinity))
	condition6 := b.or(leftZero, rightZero)
	value6 := b.fpITE(b.xor(leftPositive, rightPositive), negativeZero, positiveZero)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}

	a, err := c.Unpack(left, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := c.Unpack(right, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b = roundBuilder{c: c}
	leftLeading := b.zeroExtend(2, a.LeadingZeros)
	rightLeading := b.zeroExtend(2, d.LeadingZeros)
	leftSig := b.zeroExtend(sbits, a.Significand)
	rightSig := b.zeroExtend(sbits, d.Significand)
	leftExp := b.signExtend(2, a.Exponent)
	rightExp := b.signExtend(2, d.Exponent)
	resultSign := b.bitXor(a.Sign, d.Sign)
	resultExp := b.add(b.sub(leftExp, leftLeading), b.sub(rightExp, rightLeading))
	product := b.mul(leftSig, rightSig)
	highProduct := b.extract(2*sbits-1, sbits, product)
	lowProduct := b.extract(sbits-1, 0, product)
	var roundingBits ast.ASTHandle
	if sbits >= 4 {
		sticky := b.reductionOr(b.extract(sbits-4, 0, product))
		roundingBits = b.concat(b.extract(sbits-1, sbits-3, product), sticky)
	} else {
		roundingBits = b.concat(lowProduct, b.zero(4-sbits))
	}
	resultSig := b.concat(highProduct, roundingBits)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	result, err := c.roundBits(sort, rm, resultSign, resultSig, resultExp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, choice := range []struct {
		condition ast.ASTHandle
		value     ast.ASTHandle
	}{
		{condition6, value6},
		{rightNegativeInfinity, value5},
		{leftNegativeInfinity, value4},
		{rightPositiveInfinity, value3},
		{leftPositiveInfinity, value2},
		{condition1, nan},
	} {
		result, err = c.MkITE(choice.condition, choice.value, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

// Mul converts IEEE floating-point multiplication into bit-vector operations.
func (c *Converter) Mul(roundingMode, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.manager.GetSort(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightSort, err := c.manager.GetSort(right)
	if err != nil || rightSort != sort {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.mulBits(sort, rm, left, right)
}
