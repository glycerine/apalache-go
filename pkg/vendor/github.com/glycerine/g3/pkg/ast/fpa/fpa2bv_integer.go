package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:2796-3070.

import (
	"math/bits"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *Converter) toFPInteger(targetSort, roundingMode, value ast.ASTHandle, signed bool) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(targetSort)
	if err != nil || !floating || ebits < 2 || sbits < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bvSize, err := c.manager.BitVectorSize(value)
	if err != nil || bvSize < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	b := roundBuilder{c: c}
	zeroValue := b.zero(bvSize)
	isZero := b.equal(value, zeroValue)
	positiveZero := b.call(func() (ast.ASTHandle, error) { return c.MkSpecial(targetSort, PositiveZero) })

	sign := b.zero(1)
	magnitude := value
	if signed {
		sign = b.extract(bvSize-1, bvSize-1, value)
		negative := b.equal(sign, b.one(1))
		magnitude = b.ite(negative, b.neg(value), value)
	}
	leadingZeros := b.leading(magnitude, bvSize)
	shiftedSignificand := b.shl(magnitude, leadingZeros)
	significandSize := sbits + 4
	if significandSize <= bvSize {
		high := b.extract(bvSize-1, bvSize-significandSize+1, shiftedSignificand)
		low := b.extract(bvSize-significandSize, 0, shiftedSignificand)
		shiftedSignificand = b.concat(high, b.reductionOr(low))
	} else {
		extra := significandSize - bvSize
		shiftedSignificand = b.concat(shiftedSignificand, b.zero(extra))
		leadingZeros = b.add(b.concat(b.zero(extra), leadingZeros), b.number(extra, significandSize))
		bvSize = significandSize
	}
	if ebits+2 > bvSize {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	sourceExponent := b.sub(b.number(bvSize-2, bvSize), leadingZeros)
	exponentSize := ebits + 2
	exponent := b.extract(exponentSize-1, 0, sourceExponent)
	worstCaseSize := uint32(bits.Len32(bvSize))
	if exponentSize < worstCaseSize {
		maximumExponent := b.concat(b.zero(1), b.ones(exponentSize-1))
		extendedMaximum := b.zeroExtend(bvSize-exponentSize, maximumExponent)
		tooLarge := b.sle(b.add(extendedMaximum, b.one(bvSize)), sourceExponent)
		shiftedSignificand = b.ite(tooLarge, b.zero(significandSize), shiftedSignificand)
		exponent = b.ite(tooLarge, maximumExponent, exponent)
	}
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	converted, err := c.roundBits(targetSort, rm, sign, shiftedSignificand, exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.MkITE(isZero, positiveZero, converted)
}

// ToFPSigned converts a two's-complement bit-vector into floating point.
func (c *Converter) ToFPSigned(targetSort, roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toFPInteger(targetSort, roundingMode, value, true)
}

// ToFPUnsigned converts an unsigned bit-vector into floating point.
func (c *Converter) ToFPUnsigned(targetSort, roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toFPInteger(targetSort, roundingMode, value, false)
}
