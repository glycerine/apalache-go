package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:2703-2794.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// ToReal converts an encoded floating-point value to the source arithmetic
// expression. Under high-unspecified semantics, NaN and infinities map to 0.
func (c *Converter) toReal(declaration, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
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
	unpacked, err := c.Unpack(value, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b := roundBuilder{c: c}
	zero := b.call(func() (ast.ASTHandle, error) { return c.manager.ArithmeticRealNumeralText("0") })
	one := b.call(func() (ast.ASTHandle, error) { return c.manager.ArithmeticRealNumeralText("1") })
	two := b.call(func() (ast.ASTHandle, error) { return c.manager.ArithmeticRealNumeralText("2") })
	oneBit := b.one(1)

	realSignificand := one
	for i := int(sbits) - 2; i >= 0; i-- {
		bit := b.extract(uint32(i), uint32(i), unpacked.Significand)
		set := b.equal(bit, oneBit)
		digit := b.ite(set, one, zero)
		realSignificand = b.arithAdd(b.arithMul(realSignificand, two), digit)
	}
	denominatorText := new(big.Int).Lsh(big.NewInt(1), uint(sbits-1)).String()
	denominator := b.call(func() (ast.ASTHandle, error) {
		return c.manager.ArithmeticRealNumeralText(denominatorText)
	})
	realSignificand = b.arithDiv(realSignificand, denominator)

	exponentNegative := b.equal(b.extract(ebits-1, ebits-1, unpacked.Exponent), oneBit)
	positiveExponent := b.signExtend(1, unpacked.Exponent)
	absoluteExponent := b.ite(exponentNegative, b.neg(positiveExponent), positiveExponent)
	realExponent := zero
	for i := int(ebits); i >= 0; i-- {
		bit := b.extract(uint32(i), uint32(i), absoluteExponent)
		set := b.equal(bit, oneBit)
		digit := b.ite(set, one, zero)
		realExponent = b.arithAdd(b.arithMul(realExponent, two), digit)
	}
	realExponent = b.ite(exponentNegative, b.arithDiv(one, realExponent), realExponent)
	result := b.arithMul(realSignificand, b.arithPower(two, realExponent))
	negativeResult := b.arithNeg(result)
	negativeSign := b.equal(unpacked.Sign, oneBit)
	result = b.ite(negativeSign, negativeResult, result)

	isZero := b.call(func() (ast.ASTHandle, error) { return c.isZero(value) })
	isInfinite := b.call(func() (ast.ASTHandle, error) { return c.isInfinite(value) })
	isNaN := b.call(func() (ast.ASTHandle, error) { return c.isNaN(value) })
	unspecified := zero
	if !c.highUnspecified {
		if declaration.IsNull() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		nanWrapped := b.call(func() (ast.ASTHandle, error) { return c.nanWrap(value) })
		domainSort := b.call(func() (ast.ASTHandle, error) { return c.manager.GetSort(nanWrapped) })
		rangeSort := b.call(func() (ast.ASTHandle, error) { return c.manager.GetRange(declaration) })
		converted := b.call(func() (ast.ASTHandle, error) {
			return c.convertedUFWithDomain(declaration, []ast.ASTHandle{domainSort}, rangeSort)
		})
		unspecified = b.call(func() (ast.ASTHandle, error) {
			return c.manager.MkApp(converted, []ast.ASTHandle{nanWrapped})
		})
	}
	result = b.ite(isZero, zero, result)
	result = b.ite(isInfinite, unspecified, result)
	result = b.ite(isNaN, unspecified, result)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	return result, nil
}

func (c *Converter) ToReal(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toReal(ast.ASTHandle{}, value)
}

func (c *Converter) ToRealFor(declaration, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toReal(declaration, value)
}
