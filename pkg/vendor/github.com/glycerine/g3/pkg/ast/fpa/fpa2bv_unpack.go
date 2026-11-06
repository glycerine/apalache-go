package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:3514-3690.

import "github.com/glycerine/g3/pkg/ast"

// Unpacked is the source converter's sign, hidden-bit significand, unbiased
// exponent and normalization shift tuple.
type Unpacked struct {
	Sign, Significand, Exponent, LeadingZeros ast.ASTHandle
}

// Bias adds the IEEE exponent bias in the operand's bit-vector width.
func (c *Converter) Bias(exponent ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	width, err := c.manager.BitVectorSize(exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if width < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	zero, err := c.zero(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ones, err := c.manager.BitVectorNumeralAllOnes(width - 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bias, err := c.manager.MkBitVectorConcatPair(zero, ones)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.MkBitVectorAdd(exponent, bias)
}

// Unbias subtracts the IEEE bias using the source bit-level identity.
func (c *Converter) Unbias(exponent ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	width, err := c.manager.BitVectorSize(exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if width < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	one, err := c.one(width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	plusOne, err := c.manager.MkBitVectorAdd(exponent, one)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leading, err := c.manager.MkBitVectorExtract(width-1, width-1, plusOne)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	inverted, err := c.manager.MkBitVectorNot(leading)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rest, err := c.manager.MkBitVectorExtract(width-2, 0, plusOne)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.MkBitVectorConcatPair(inverted, rest)
}

// LeadingZeros builds the source divide-and-conquer leading-zero counter.
func (c *Converter) LeadingZeros(value ast.ASTHandle, resultWidth uint32) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if resultWidth == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	width, err := c.manager.BitVectorSize(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if width == 1 {
		zero1, err := c.zero(1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		isZero, err := c.boolEq(value, zero1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		one, err := c.one(resultWidth)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		zero, err := c.zero(resultWidth)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return c.boolITE(isZero, one, zero)
	}
	high, err := c.manager.MkBitVectorExtract(width-1, width/2, value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	low, err := c.manager.MkBitVectorExtract(width/2-1, 0, value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	highZeros, err := c.LeadingZeros(high, resultWidth)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lowZeros, err := c.LeadingZeros(low, resultWidth)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	highWidth, err := c.manager.BitVectorSize(high)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zeroHigh, err := c.zero(highWidth)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	highIsZero, err := c.boolEq(high, zeroHigh)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	highCount, err := c.manager.BitVectorNumeralUnsigned(highWidth, resultWidth)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sum, err := c.manager.MkBitVectorAdd(highCount, lowZeros)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolITE(highIsZero, sum, highZeros)
}

// Unpack inserts the hidden significand bit and unbiases the exponent. When
// normalize is true, subnormal significands are shifted left and the shift is
// returned in LeadingZeros.
func (c *Converter) Unpack(value ast.ASTHandle, normalize bool) (Unpacked, error) {
	if err := c.open(); err != nil {
		return Unpacked{}, err
	}
	sort, err := c.manager.GetSort(value)
	if err != nil {
		return Unpacked{}, err
	}
	ebits, sbits, matched, err := c.manager.FloatingPointSortShape(sort)
	if err != nil {
		return Unpacked{}, err
	}
	if !matched {
		return Unpacked{}, ast.ErrASTDomain
	}
	sign, biasedExponent, trailing, err := c.SplitFP(value)
	if err != nil {
		return Unpacked{}, err
	}
	normal, err := c.isNormal(value)
	if err != nil {
		return Unpacked{}, err
	}
	one1, err := c.one(1)
	if err != nil {
		return Unpacked{}, err
	}
	normalSignificand, err := c.manager.MkBitVectorConcatPair(one1, trailing)
	if err != nil {
		return Unpacked{}, err
	}
	normalExponent, err := c.Unbias(biasedExponent)
	if err != nil {
		return Unpacked{}, err
	}
	denormalSignificand, err := c.manager.MkBitVectorZeroExtend(1, trailing)
	if err != nil {
		return Unpacked{}, err
	}
	oneExponent, err := c.one(ebits)
	if err != nil {
		return Unpacked{}, err
	}
	denormalExponent, err := c.Unbias(oneExponent)
	if err != nil {
		return Unpacked{}, err
	}
	zeroExponent, err := c.zero(ebits)
	if err != nil {
		return Unpacked{}, err
	}
	leadingZeros := zeroExponent
	if normalize {
		zeroSignificand, err := c.zero(sbits)
		if err != nil {
			return Unpacked{}, err
		}
		significandZero, err := c.boolEq(zeroSignificand, denormalSignificand)
		if err != nil {
			return Unpacked{}, err
		}
		count, err := c.LeadingZeros(denormalSignificand, ebits)
		if err != nil {
			return Unpacked{}, err
		}
		normalOrZero, err := c.boolOr(normal, significandZero)
		if err != nil {
			return Unpacked{}, err
		}
		leadingZeros, err = c.boolITE(normalOrZero, zeroExponent, count)
		if err != nil {
			return Unpacked{}, err
		}
		shift, err := c.boolITE(significandZero, zeroExponent, leadingZeros)
		if err != nil {
			return Unpacked{}, err
		}
		if ebits <= sbits {
			amount, err := c.manager.MkBitVectorZeroExtend(sbits-ebits, shift)
			if err != nil {
				return Unpacked{}, err
			}
			denormalSignificand, err = c.manager.MkBitVectorSHL(denormalSignificand, amount)
			if err != nil {
				return Unpacked{}, err
			}
		} else {
			zeroHigh, err := c.zero(ebits - sbits)
			if err != nil {
				return Unpacked{}, err
			}
			high, err := c.manager.MkBitVectorExtract(ebits-1, sbits, shift)
			if err != nil {
				return Unpacked{}, err
			}
			highZero, err := c.boolEq(zeroHigh, high)
			if err != nil {
				return Unpacked{}, err
			}
			short, err := c.manager.MkBitVectorExtract(sbits-1, 0, shift)
			if err != nil {
				return Unpacked{}, err
			}
			maximum, err := c.manager.BitVectorNumeralUnsigned(sbits, sbits)
			if err != nil {
				return Unpacked{}, err
			}
			amount, err := c.boolITE(highZero, short, maximum)
			if err != nil {
				return Unpacked{}, err
			}
			denormalSignificand, err = c.manager.MkBitVectorSHL(denormalSignificand, amount)
			if err != nil {
				return Unpacked{}, err
			}
		}
	}
	significand, err := c.boolITE(normal, normalSignificand, denormalSignificand)
	if err != nil {
		return Unpacked{}, err
	}
	exponent, err := c.boolITE(normal, normalExponent, denormalExponent)
	if err != nil {
		return Unpacked{}, err
	}
	return Unpacked{Sign: sign, Significand: significand, Exponent: exponent, LeadingZeros: leadingZeros}, nil
}
