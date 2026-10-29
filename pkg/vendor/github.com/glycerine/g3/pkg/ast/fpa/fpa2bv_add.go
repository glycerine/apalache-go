package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:371-650.

import (
	"math/bits"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *Converter) addCore(sbits, ebits uint32, cSign, cSig, cExp, dSign, dSig, dExp ast.ASTHandle) (sign, significand, exponent ast.ASTHandle, result error) {
	b := roundBuilder{c: c}
	exponentDelta := b.sub(cExp, dExp)
	if uint32(bits.Len32(sbits+2)-1) < ebits+2 {
		cap := b.number(sbits+2, ebits+2)
		extended := b.zeroExtend(2, exponentDelta)
		exponentDelta = b.ite(b.ule(cap, extended), cap, extended)
		exponentDelta = b.extract(ebits-1, 0, exponentDelta)
	}
	cSig = b.concat(cSig, b.zero(3))
	dSig = b.concat(dSig, b.zero(3))
	wideWidth := 2 * (sbits + 3)
	if wideWidth < ebits {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, ast.ErrASTDomain
	}
	wideD := b.concat(dSig, b.zero(sbits+3))
	shift := exponentDelta
	if wideWidth > ebits {
		shift = b.concat(b.zero(wideWidth-ebits), exponentDelta)
	}
	shiftedWide := b.lshr(wideD, shift)
	shiftedD := b.extract(wideWidth-1, sbits+3, shiftedWide)
	stickyRaw := b.extract(sbits+2, 0, shiftedWide)
	zeroSig := b.zero(sbits + 3)
	sticky := b.ite(b.equal(stickyRaw, zeroSig), zeroSig, b.one(sbits+3))
	shiftedD = b.bitOr(shiftedD, sticky)
	sameSign := b.equal(cSign, dSign)
	cSig = b.zeroExtend(2, cSig)
	shiftedD = b.zeroExtend(2, shiftedD)
	sum := b.ite(sameSign, b.add(cSig, shiftedD), b.sub(cSig, shiftedD))
	sumSign := b.extract(sbits+4, sbits+4, sum)
	negativeSum := b.neg(sum)
	notCSign := b.bitNot(cSign)
	notDSign := b.bitNot(dSign)
	notSumSign := b.bitNot(sumSign)
	firstSign := b.bitAnd(notCSign, dSign, sumSign)
	secondSign := b.bitAnd(cSign, notDSign, notSumSign)
	thirdSign := b.bitAnd(cSign, dSign)
	sign = b.bitOr(firstSign, secondSign, thirdSign)
	absSum := b.ite(b.equal(sumSign, b.one(1)), negativeSum, sum)
	significand = b.extract(sbits+3, 0, absSum)
	exponent = b.signExtend(2, cExp)
	if b.err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, b.err
	}
	return sign, significand, exponent, nil
}

func (c *Converter) addBits(sort, rm, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	nan, err := c.MkSpecial(sort, NaN)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeZero, err := c.MkSpecial(sort, NegativeZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveZero, err := c.MkSpecial(sort, PositiveZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b := roundBuilder{c: c}
	leftNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(left) })
	rightNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(right) })
	leftZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(left) })
	rightZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(right) })
	leftNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(left) })
	rightNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(right) })
	leftInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(left) })
	rightInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(right) })

	condition1 := b.or(leftNaN, rightNaN)
	signsXor := b.xor(leftNegative, rightNegative)
	value2 := b.fpITE(b.and(rightInfinite, signsXor), nan, left)
	value3 := b.fpITE(b.and(leftInfinite, signsXor), nan, right)
	condition4 := b.and(leftZero, rightZero)
	roundTowardNegative := b.mode(rm, 3)
	negativeCondition := b.or(b.and(leftNegative, rightNegative), b.and(roundTowardNegative, signsXor))
	value4 := b.fpITE(negativeCondition, negativeZero, positiveZero)
	value4 = b.fpITE(b.and(leftNegative, rightNegative), left, value4)

	a, e := c.Unpack(left, false)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	d, e := c.Unpack(right, false)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	swap := b.sle(a.Exponent, d.Exponent)
	cSign := b.ite(swap, d.Sign, a.Sign)
	cSig := b.ite(swap, d.Significand, a.Significand)
	cExp := b.ite(swap, d.Exponent, a.Exponent)
	dSign := b.ite(swap, a.Sign, d.Sign)
	dSig := b.ite(swap, a.Significand, d.Significand)
	dExp := b.ite(swap, a.Exponent, d.Exponent)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	resultSign, resultSig, resultExp, err := c.addCore(sbits, ebits, cSign, cSig, cExp, dSign, dSig, dExp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zeroSignificand, err := c.zero(sbits + 4)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	isZeroSignificand, err := c.boolEq(resultSig, zeroSignificand)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zeroCase, err := c.MkITE(roundTowardNegative, negativeZero, positiveZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rounded, err := c.roundBits(sort, rm, resultSign, resultSig, resultExp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := c.MkITE(isZeroSignificand, zeroCase, rounded)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, choice := range []struct {
		condition ast.ASTHandle
		value     ast.ASTHandle
	}{
		{rightZero, left},
		{leftZero, right},
		{condition4, value4},
		{rightInfinite, value3},
		{leftInfinite, value2},
		{condition1, nan},
	} {
		result, err = c.MkITE(choice.condition, choice.value, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

// Add converts IEEE floating-point addition into bit-vector operations.
func (c *Converter) Add(roundingMode, left, right ast.ASTHandle) (ast.ASTHandle, error) {
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
	return c.addBits(sort, rm, left, right)
}

// Sub converts subtraction through the source add-of-negation path.
func (c *Converter) Sub(roundingMode, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	negative, err := c.Negate(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.Add(roundingMode, left, negative)
}
