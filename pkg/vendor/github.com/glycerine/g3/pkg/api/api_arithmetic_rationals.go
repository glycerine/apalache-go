package api

// Source: src/api/api_arith.cpp:177-198.

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (c *SMTContext) arithmeticRationalPart(a ast.ASTHandle, numerator bool) (ast.ASTHandle, error) {
	m, _, e := c.arithmeticManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	text, _, yes, e := m.ArithmeticNumeralText(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if !yes {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	value := r.Denom()
	if numerator {
		value = r.Num()
	}
	return m.ArithmeticIntegerNumeralText(value.String())
}
func (c *SMTContext) Numerator(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticRationalPart(a, true)
}
func (c *SMTContext) Denominator(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticRationalPart(a, false)
}
