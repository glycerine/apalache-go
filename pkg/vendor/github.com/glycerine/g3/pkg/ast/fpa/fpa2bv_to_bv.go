package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:3123-3327.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) toBV(declaration ast.ASTHandle, width uint32, roundingMode, value ast.ASTHandle, signed bool) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if width == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.manager.GetSort(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating || ebits < 2 || sbits < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	b := roundBuilder{c: c}
	zero1 := b.zero(1)
	one1 := b.one(1)
	unspecified := b.zero(width)
	if !c.highUnspecified {
		if declaration.IsNull() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		nanWrapped := b.call(func() (ast.ASTHandle, error) { return c.nanWrap(value) })
		rmSort := b.call(func() (ast.ASTHandle, error) { return c.manager.GetSort(rm) })
		wrappedSort := b.call(func() (ast.ASTHandle, error) { return c.manager.GetSort(nanWrapped) })
		rangeSort := b.call(func() (ast.ASTHandle, error) { return c.manager.GetRange(declaration) })
		converted := b.call(func() (ast.ASTHandle, error) {
			return c.convertedUFWithDomain(declaration, []ast.ASTHandle{rmSort, wrappedSort}, rangeSort)
		})
		unspecified = b.call(func() (ast.ASTHandle, error) {
			return c.manager.MkApp(converted, []ast.ASTHandle{rm, nanWrapped})
		})
	}
	isNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(value) })
	isInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(value) })
	isZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(value) })
	isNegative := b.call(func() (ast.ASTHandle, error) { return c.isNegativeSign(value) })

	unpacked := Unpacked{}
	unpacked, b.err = c.Unpack(value, true)
	significand := unpacked.Significand
	significandSize := sbits
	if significandSize < width+3 {
		significand = b.concat(significand, b.zero(width-significandSize+3))
		significandSize = width + 3
	}
	exponentMinusLeading := b.sub(b.signExtend(2, unpacked.Exponent), b.zeroExtend(2, unpacked.LeadingZeros))
	bigSignificand := b.concat(b.zeroExtend(width+2, significand), zero1)
	bigSize := significandSize + width + 3
	negativeShift := b.sle(exponentMinusLeading, b.zero(ebits+2))
	shift := b.ite(negativeShift, b.neg(exponentMinusLeading), exponentMinusLeading)
	if ebits+2 < bigSize {
		shift = b.zeroExtend(bigSize-ebits-2, shift)
	} else if ebits+2 > bigSize {
		upper := b.extract(ebits+1, bigSize, shift)
		lower := b.extract(bigSize-1, 0, shift)
		shift = b.ite(b.equal(upper, b.zero(ebits+2-bigSize)), lower, b.number(bigSize-1, bigSize))
	}
	shiftLimit := b.number(width+2, bigSize)
	shift = b.ite(b.ule(shift, shiftLimit), shift, shiftLimit)
	shifted := b.ite(negativeShift, b.lshr(bigSignificand, shift), b.shl(bigSignificand, shift))
	intPart := b.extract(bigSize-1, bigSize-(width+3), shifted)
	last := b.extract(bigSize-(width+3), bigSize-(width+3), shifted)
	round := b.extract(bigSize-(width+4), bigSize-(width+4), shifted)
	stickies := b.extract(bigSize-(width+5), 0, shifted)
	sticky := b.reductionOr(stickies)
	increment := b.decision(rm, unpacked.Sign, last, round, sticky)
	preRounded := b.add(intPart, b.zeroExtend(width+2, increment))
	incremented := b.equal(increment, one1)
	preRoundedZero := b.equal(preRounded, b.zero(width+3))
	overflow := b.and(incremented, preRoundedZero)

	var inRange ast.ASTHandle
	if !signed {
		upper := b.zeroExtend(3, b.neg(b.one(width)))
		negativeAllowedZero := b.or(b.not(isNegative), preRoundedZero)
		inRange = b.and(negativeAllowedZero, b.not(overflow), b.ule(preRounded, upper))
	} else {
		lower := b.signExtend(3, b.concat(one1, b.zero(width-1)))
		upper := b.zeroExtend(4, b.neg(b.one(width-1)))
		overflow = b.or(overflow, b.sle(preRounded, b.neg(b.one(width+3))))
		inRange = b.and(b.not(overflow), b.sle(lower, preRounded), b.sle(preRounded, upper))
		preRounded = b.ite(isNegative, b.neg(preRounded), preRounded)
	}
	rounded := b.extract(width-1, 0, preRounded)
	result := b.ite(b.not(inRange), unspecified, rounded)
	result = b.ite(isZero, unspecified, result)
	result = b.ite(b.or(isNaN, isInfinite), unspecified, result)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	return result, nil
}

// ToUBV converts floating point to an unsigned bit-vector.
func (c *Converter) ToUBV(width uint32, roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toBV(ast.ASTHandle{}, width, roundingMode, value, false)
}

// ToSBV converts floating point to a signed bit-vector.
func (c *Converter) ToSBV(width uint32, roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toBV(ast.ASTHandle{}, width, roundingMode, value, true)
}

func (c *Converter) ToUBVFor(declaration ast.ASTHandle, width uint32, roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toBV(declaration, width, roundingMode, value, false)
}

func (c *Converter) ToSBVFor(declaration ast.ASTHandle, width uint32, roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toBV(declaration, width, roundingMode, value, true)
}
