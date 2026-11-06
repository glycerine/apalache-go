package api

// Source: src/api/api_ast.cpp:46-223.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *SMTContext) IntegerSymbol(value uint32) (usymbol.Symbol, error) {
	if _, e := c.Manager(); e != nil {
		return usymbol.Symbol{}, e
	}
	return usymbol.Numerical(value), nil
}
func (c *SMTContext) StringSymbol(value string) (usymbol.Symbol, error) {
	m, e := c.Manager()
	if e != nil {
		return usymbol.Symbol{}, e
	}
	if value == "" || value[0] == 0 {
		return usymbol.Symbol{}, nil
	}
	return m.Symbols.Intern(value)
}
func (c *SMTContext) SymbolString(value usymbol.Symbol) (string, error) {
	m, e := c.Manager()
	if e != nil {
		return "", e
	}
	if value.IsNumerical() {
		n, e := value.Number()
		if e != nil {
			return "", e
		}
		return strconv.FormatUint(uint64(n), 10), nil
	}
	text, _, e := m.Symbols.BareString(value)
	return text, e
}
func (c *SMTContext) UninterpretedSort(name string) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	symbol, e := c.StringSymbol(name)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkUninterpretedSort(symbol, nil)
}
func (c *SMTContext) Function(name string, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	symbol, e := c.StringSymbol(name)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFuncDecl(symbol, domain, rangeSort, ast.ASTFuncDeclInfo{})
}
func (c *SMTContext) Apply(declaration ast.ASTHandle, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkApp(declaration, args)
}
func (c *SMTContext) Constant(name string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	symbol, e := c.StringSymbol(name)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkConst(symbol, sort)
}
func (c *SMTContext) FreshFunction(prefix string, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	symbol, e := m.Symbols.Intern(prefix)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFreshFuncDecl(symbol, usymbol.Symbol{}, domain, rangeSort)
}
func (c *SMTContext) FreshConstant(prefix string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFreshConst(prefix, sort)
}
func (c *SMTContext) BoolSort() (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkBoolSort()
}
func (c *SMTContext) Bool(value bool) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkBoolVal(value)
}
func (c *SMTContext) Not(a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkNot(a)
}
func (c *SMTContext) Equal(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpEq, nil, []ast.ASTHandle{a, b})
}
func (c *SMTContext) Distinct(args ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkDistinct(args)
}
func (c *SMTContext) Iff(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkIff(a, b)
}
func (c *SMTContext) Implies(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkImplies(a, b)
}
func (c *SMTContext) Xor(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkXor(a, b)
}
func (c *SMTContext) And(args ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkAndArgs(args)
}
func (c *SMTContext) Or(args ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkOrArgs(args)
}
func (c *SMTContext) Interpolant(a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkInterp(a)
}
func (c *SMTContext) Ite(condition, thenValue, elseValue ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkIte(condition, thenValue, elseValue)
}
