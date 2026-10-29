package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:973-1166.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *Converter) positiveTwo(sort ast.ASTHandle) (ast.ASTHandle, error) {
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	zero1, err := c.zero(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	biasLow, err := c.manager.BitVectorNumeralAllOnes(ebits - 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bias, err := c.manager.MkBitVectorConcatPair(zero1, biasLow)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := c.one(ebits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := c.manager.MkBitVectorAdd(bias, one)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	significand, err := c.zero(sbits - 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.MkFP(zero1, exponent, significand)
}

// Rem converts the IEEE nearest-integer remainder operation into bit-vector
// arithmetic. It preserves the dividend sign when the result is zero.
func (c *Converter) Rem(left, right ast.ASTHandle) (ast.ASTHandle, error) {
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
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating || ebits > 32 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	specials := make([]ast.ASTHandle, 3)
	for i, kind := range []SpecialValue{NaN, NegativeZero, PositiveZero} {
		specials[i], err = c.MkSpecial(sort, kind)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	nan, negativeZero, positiveZero := specials[0], specials[1], specials[2]
	b := roundBuilder{c: c}
	leftNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(left) })
	rightNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(right) })
	leftZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(left) })
	rightZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(right) })
	leftPositive := b.call(func() (ast.ASTHandle, error) { return c.isPositiveSign(left) })
	rightPositive := b.call(func() (ast.ASTHandle, error) { return c.isPositiveSign(right) })
	rightNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(right) })
	leftInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(left) })
	rightInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(right) })
	condition1 := b.or(leftNaN, rightNaN)
	_, leftExponent, _, splitErr := c.SplitFP(left)
	if splitErr != nil {
		return ast.ASTHandle{}, splitErr
	}
	_, rightExponent, _, splitErr := c.SplitFP(right)
	if splitErr != nil {
		return ast.ASTHandle{}, splitErr
	}
	rightExponentMinusOne := b.sub(rightExponent, b.one(ebits))
	exponentEarly := b.and(b.not(b.equal(rightExponent, b.zero(ebits))), b.ult(leftExponent, rightExponentMinusOne))
	if b.err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder setup: %w", b.err)
	}

	a, err := c.Unpack(left, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := c.Unpack(right, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	maxDifference64 := (uint64(1) << ebits) - 3
	shiftWidth64 := maxDifference64 + uint64(sbits) + 3
	if shiftWidth64 > 1<<32-1 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	maxDifference := uint32(maxDifference64)
	shiftWidth := uint32(shiftWidth64)
	if shiftWidth < ebits+2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	b = roundBuilder{c: c}
	leftExp := b.signExtend(2, a.Exponent)
	rightExp := b.signExtend(2, d.Exponent)
	leftLeading := b.zeroExtend(2, a.LeadingZeros)
	rightLeading := b.zeroExtend(2, d.LeadingZeros)
	exponentDifference := b.sub(b.sub(leftExp, leftLeading), b.sub(rightExp, rightLeading))
	negativeDifference := b.neg(exponentDifference)
	differenceNegative := b.sle(exponentDifference, b.zero(ebits+2))
	leftSig := b.concat(b.zeroExtend(maxDifference, a.Significand), b.zero(3))
	rightSig := b.concat(b.zeroExtend(maxDifference, d.Significand), b.zero(3))
	extension := shiftWidth - (ebits + 2)
	leftShift := b.zeroExtend(extension, exponentDifference)
	rightShift := b.zeroExtend(extension, negativeDifference)
	shifted := b.ite(differenceNegative, b.ashr(leftSig, rightShift), b.shl(leftSig, leftShift))
	hugeRemainder := b.urem(shifted, rightSig)
	roundedSign := a.Sign
	roundedExponent := b.sub(rightExp, rightLeading)
	roundedSignificand := b.extract(sbits+3, 0, hugeRemainder)
	rne := b.zero(3)
	if b.err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder core: %w", b.err)
	}
	rounded, err := c.roundBits(sort, rne, roundedSign, roundedSignificand, roundedExponent)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder rounding: %w", err)
	}
	two, err := c.positiveTwo(sort)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder two: %w", err)
	}
	half, err := c.divBits(sort, rne, right, two)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder half divisor: %w", err)
	}
	negativeHalf, err := c.Negate(half)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder negative half: %w", err)
	}
	halfZero, err := c.isZero(half)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder half zero: %w", err)
	}
	halfNonzero, err := c.boolNot(halfZero)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder half nonzero: %w", err)
	}
	geHalf, err := c.Compare(ast.FPAOpGe, rounded, half)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder >= half: %w", err)
	}
	gtNegativeHalf, err := c.Compare(ast.FPAOpGt, rounded, negativeHalf)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder > negative half: %w", err)
	}
	leHalf, err := c.Compare(ast.FPAOpLe, rounded, half)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder <= half: %w", err)
	}
	ltNegativeHalf, err := c.Compare(ast.FPAOpLt, rounded, negativeHalf)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder < negative half: %w", err)
	}
	negativeRight, err := c.Negate(right)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder negative divisor: %w", err)
	}
	roundedMinusRight, err := c.addBits(sort, rne, rounded, negativeRight)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder subtract divisor: %w", err)
	}
	roundedPlusRight, err := c.addBits(sort, rne, rounded, right)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder add divisor: %w", err)
	}
	b = roundBuilder{c: c}
	subtractCondition := b.and(halfNonzero, b.or(b.and(rightPositive, geHalf), b.and(rightNegative, leHalf)))
	addCondition := b.and(halfNonzero, b.or(b.and(rightPositive, ltNegativeHalf), b.and(rightNegative, gtNegativeHalf)))
	result := b.fpITE(addCondition, roundedPlusRight, rounded)
	result = b.fpITE(subtractCondition, roundedMinusRight, result)
	for _, choice := range []struct {
		condition ast.ASTHandle
		value     ast.ASTHandle
	}{
		{exponentEarly, left},
		{leftZero, positiveZero},
		{rightZero, nan},
		{rightInfinite, left},
		{leftInfinite, nan},
		{condition1, nan},
	} {
		result = b.fpITE(choice.condition, choice.value, result)
	}
	resultZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(result) })
	signedZero := b.fpITE(leftPositive, positiveZero, negativeZero)
	result = b.fpITE(resultZero, signedZero, result)
	if b.err != nil {
		return ast.ASTHandle{}, fmt.Errorf("remainder selection: %w", b.err)
	}
	return result, nil
}
