package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:820-971.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) divBits(sort, rm, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating || ebits > sbits {
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
	value2 := b.fpITE(rightInfinite, nan, b.fpITE(rightPositive, positiveInfinity, negativeInfinity))
	signsXor := b.xor(leftPositive, rightPositive)
	signedZero := b.fpITE(signsXor, negativeZero, positiveZero)
	value3 := b.fpITE(leftInfinite, nan, signedZero)
	value4 := b.fpITE(rightInfinite, nan, b.fpITE(rightPositive, negativeInfinity, positiveInfinity))
	value5 := b.fpITE(leftInfinite, nan, signedZero)
	signedInfinity := b.fpITE(signsXor, negativeInfinity, positiveInfinity)
	value6 := b.fpITE(leftZero, nan, signedInfinity)
	value7 := signedZero
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
	extraBits := sbits + 2
	leftSig := b.concat(a.Significand, b.zero(sbits+extraBits))
	rightSig := b.zeroExtend(sbits+extraBits, d.Significand)
	leftExp := b.signExtend(2, a.Exponent)
	rightExp := b.signExtend(2, d.Exponent)
	resultSign := b.bitXor(a.Sign, d.Sign)
	leftLeading := b.zeroExtend(2, a.LeadingZeros)
	rightLeading := b.zeroExtend(2, d.LeadingZeros)
	resultExp := b.sub(b.sub(leftExp, leftLeading), b.sub(rightExp, rightLeading))
	quotient := b.udiv(leftSig, rightSig)
	sticky := b.reductionOr(b.extract(extraBits-2, 0, quotient))
	resultSig := b.concat(b.extract(extraBits+sbits+1, extraBits-1, quotient), sticky)
	resultLeading := b.leading(resultSig, sbits+4)
	shiftAmount := b.sub(resultLeading, b.one(sbits+4))
	shiftCondition := b.ule(resultLeading, b.one(sbits+4))
	shiftedSig := b.shl(resultSig, shiftAmount)
	shiftedExp := b.sub(resultExp, b.extract(ebits+1, 0, shiftAmount))
	resultSig = b.ite(shiftCondition, resultSig, shiftedSig)
	resultExp = b.ite(shiftCondition, resultExp, shiftedExp)
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
		{leftZero, value7},
		{rightZero, value6},
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

// Div converts IEEE floating-point division into bit-vector operations.
func (c *Converter) Div(roundingMode, left, right ast.ASTHandle) (ast.ASTHandle, error) {
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
	return c.divBits(sort, rm, left, right)
}
