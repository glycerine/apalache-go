package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:1693-1840.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) sqrtBits(sort, rm, value ast.ASTHandle) (ast.ASTHandle, error) {
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	b := roundBuilder{c: c}
	nan := b.call(func() (ast.ASTHandle, error) { return c.MkSpecial(sort, NaN) })
	valueNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(value) })
	valueZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(value) })
	valuePositive := b.call(func() (ast.ASTHandle, error) { return c.isPositiveSign(value) })
	valueInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(value) })
	positiveInfinity := b.and(valueInfinite, valuePositive)
	valueNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(value) })
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	unpacked, err := c.Unpack(value, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b = roundBuilder{c: c}
	zero1, one1 := b.zero(1), b.one(1)
	realExponent := b.sub(b.signExtend(1, unpacked.Exponent), b.zeroExtend(1, unpacked.LeadingZeros))
	resultExponent := b.signExtend(2, b.extract(ebits, 1, realExponent))
	exponentOdd := b.equal(b.extract(0, 0, realExponent), one1)
	significandPrime := b.ite(exponentOdd, b.concat(unpacked.Significand, zero1), b.concat(zero1, unpacked.Significand))
	q := b.concat(zero1, b.concat(one1, b.zero(sbits+3)))
	r := b.sub(b.concat(significandPrime, b.zero(4)), q)
	s := q
	for i := uint32(0); i < sbits+3; i++ {
		s = b.concat(zero1, b.extract(sbits+4, 1, s))
		twoQPlusS := b.add(b.concat(q, zero1), b.concat(zero1, s))
		t := b.sub(b.concat(r, zero1), twoQPlusS)
		negative := b.equal(b.extract(sbits+5, sbits+5, t), one1)
		q = b.ite(negative, q, b.bitOr(q, s))
		shiftedR := b.concat(b.extract(sbits+3, 0, r), zero1)
		r = b.ite(negative, shiftedR, b.extract(sbits+4, 0, t))
	}
	exact := b.equal(r, b.zero(sbits+5))
	last := b.extract(0, 0, q)
	rest := b.extract(sbits+3, 1, q)
	restExtended := b.zeroExtend(1, rest)
	sticky := b.ite(exact, b.zeroExtend(sbits+3, last), b.one(sbits+4))
	resultSignificand := b.bitOr(restExtended, sticky)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	rounded, err := c.roundBits(sort, rm, zero1, resultSignificand, resultExponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b = roundBuilder{c: c}
	result := b.fpITE(valueNegative, nan, rounded)
	result = b.fpITE(valueZero, value, result)
	result = b.fpITE(positiveInfinity, value, result)
	result = b.fpITE(valueNaN, value, result)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	return result, nil
}

// Sqrt converts IEEE floating-point square root into bit-vector operations.
func (c *Converter) Sqrt(roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
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
	return c.sqrtBits(sort, rm, value)
}
