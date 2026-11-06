package api

// Source: src/api/api_fpa.cpp:938-1001,1160-1198.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) FloatingPointNumeralSignBit(value ast.ASTHandle) (ast.ASTHandle, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.BitVectorSort(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sign := int64(0)
	if parts.Sign {
		sign = 1
	}
	return c.BitVectorValue(sign, sort)
}

func (c *SMTContext) FloatingPointNumeralSignificandBitVector(value ast.ASTHandle) (ast.ASTHandle, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if parts.SignificandBits < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	sort, err := c.BitVectorSort(parts.SignificandBits - 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	significand := parts.Significand
	if parts.Infinite {
		significand = "0"
	}
	return c.Numeral(significand, sort)
}

func (c *SMTContext) FloatingPointNumeralExponentBitVector(value ast.ASTHandle, biased bool) (ast.ASTHandle, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := floatingPointNumeralExponent(parts, biased)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.BitVectorSort(parts.ExponentBits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.BitVectorValue(exponent, sort)
}
