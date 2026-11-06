package api

// Source: src/api/api_arith.cpp:25-132.

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (c *SMTContext) arithmeticManager() (*ast.ASTManager, ast.FamilyID, error) {
	m, e := c.Manager()
	if e != nil {
		return nil, ast.NullFamilyID, e
	}
	family, e := m.RegisterArithmeticPlugin()
	return m, family, e
}
func (c *SMTContext) IntegerSort() (ast.ASTHandle, error) {
	m, f, e := c.arithmeticManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFamilySort(f, 1, nil)
}
func (c *SMTContext) RealSort() (ast.ASTHandle, error) {
	m, f, e := c.arithmeticManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFamilySort(f, 0, nil)
}
func (c *SMTContext) Real(num, den int64) (ast.ASTHandle, error) {
	if den == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m, _, e := c.arithmeticManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.ArithmeticNumeralRat(big.NewRat(num, den), false)
}
func (c *SMTContext) arithmeticApp(kind int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, f, e := c.arithmeticManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFamilyApp(f, kind, nil, args)
}
func (c *SMTContext) ArithmeticAdd(args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(6, args...)
}
func (c *SMTContext) ArithmeticMul(args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(9, args...)
}
func (c *SMTContext) ArithmeticPower(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(18, a, b)
}
func (c *SMTContext) ArithmeticMod(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(13, a, b)
}
func (c *SMTContext) ArithmeticRem(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(12, a, b)
}
func (c *SMTContext) ArithmeticDiv(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, f, e := c.arithmeticManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	sort, e := m.GetSort(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	kind, e := m.GetDeclKind(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	op := int32(10)
	if kind == 1 {
		op = 11
	}
	return m.MkFamilyApp(f, op, nil, []ast.ASTHandle{a, b})
}
func (c *SMTContext) ArithmeticLT(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(4, a, b)
}
func (c *SMTContext) ArithmeticGT(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(5, a, b)
}
func (c *SMTContext) ArithmeticLE(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(2, a, b)
}
func (c *SMTContext) ArithmeticGE(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(3, a, b)
}
func (c *SMTContext) IntegerToReal(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(14, a)
}
func (c *SMTContext) RealToInteger(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(15, a)
}
func (c *SMTContext) IsIntegerValue(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(16, a)
}
func (c *SMTContext) ArithmeticSub(args ...ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	r := args[0]
	for _, arg := range args[1:] {
		var e error
		r, e = c.arithmeticApp(7, r, arg)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return r, nil
}
func (c *SMTContext) ArithmeticNeg(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticApp(8, a)
}
