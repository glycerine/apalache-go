package api

// Source: src/api/api_ast.cpp:225-385,549-604,656-670,880-884,1212-1228.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type APIASTKind uint8

const (
	APIASTUnknown APIASTKind = iota
	APIASTNumeral
	APIASTApplication
	APIASTVariable
	APIASTQuantifier
	APIASTSort
	APIASTFunctionDeclaration
)

func (c *SMTContext) ASTEqual(a, b ast.ASTHandle) (bool, error) {
	m, e := c.Manager()
	if e != nil {
		return false, e
	}
	if _, e = m.GetKind(a); e != nil {
		return false, e
	}
	if _, e = m.GetKind(b); e != nil {
		return false, e
	}
	return a == b, nil
}
func (c *SMTContext) ASTID(a ast.ASTHandle) (uint32, error) {
	m, e := c.Manager()
	if e != nil {
		return 0, e
	}
	return m.GetASTID(a)
}
func (c *SMTContext) ASTHash(a ast.ASTHandle) (uint32, error) {
	m, e := c.Manager()
	if e != nil {
		return 0, e
	}
	return m.GetHash(a)
}
func (c *SMTContext) ASTNodeKind(a ast.ASTHandle) (ast.ASTKind, error) {
	m, e := c.Manager()
	if e != nil {
		return 0, e
	}
	return m.GetKind(a)
}
func (c *SMTContext) APIKind(a ast.ASTHandle) (APIASTKind, error) {
	kind, e := c.ASTNodeKind(a)
	if e != nil {
		return APIASTUnknown, e
	}
	switch kind {
	case ast.ASTApp:
		m, e := c.Manager()
		if e != nil {
			return APIASTUnknown, e
		}
		yes, e := apiASTNumeralKind(m, a)
		if e != nil {
			return APIASTUnknown, e
		}
		if yes {
			return APIASTNumeral, nil
		}
		return APIASTApplication, nil
	case ast.ASTVar:
		return APIASTVariable, nil
	case ast.ASTQuantifier:
		return APIASTQuantifier, nil
	case ast.ASTSort:
		return APIASTSort, nil
	case ast.ASTFuncDecl:
		return APIASTFunctionDeclaration, nil
	}
	return APIASTUnknown, nil
}
func (c *SMTContext) IsApplication(a ast.ASTHandle) (bool, error) {
	if _, err := c.Manager(); err != nil {
		return false, err
	}
	if a.IsNull() {
		return false, nil
	}
	kind, e := c.ASTNodeKind(a)
	return kind == ast.ASTApp, e
}
func (c *SMTContext) IsWellSorted(a ast.ASTHandle) (bool, error) {
	m, e := c.Manager()
	if e != nil {
		return false, e
	}
	return m.IsWellSorted(a, nil)
}
func (c *SMTContext) ApplicationDeclaration(a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.GetDecl(a)
}
func (c *SMTContext) ApplicationArgumentCount(a ast.ASTHandle) (uint32, error) {
	m, e := c.Manager()
	if e != nil {
		return 0, e
	}
	return m.GetNumArgs(a)
}
func (c *SMTContext) ApplicationArgument(a ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.GetArg(a, index)
}
func (c *SMTContext) DeclarationName(d ast.ASTHandle) (string, error) {
	m, e := c.Manager()
	if e != nil {
		return "", e
	}
	name, e := m.GetASTName(d)
	if e != nil {
		return "", e
	}
	return m.Symbols.Display(name)
}
func (c *SMTContext) SortName(s ast.ASTHandle) (string, error) { return c.DeclarationName(s) }
func (c *SMTContext) SortOf(a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.GetSort(a)
}
func (c *SMTContext) DeclarationArity(d ast.ASTHandle) (uint32, error) {
	m, e := c.Manager()
	if e != nil {
		return 0, e
	}
	return m.GetArity(d)
}
func (c *SMTContext) DeclarationDomain(d ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.GetDomain(d, index)
}
func (c *SMTContext) DeclarationRange(d ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.Manager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.GetRange(d)
}
func (c *SMTContext) DeclarationKind(d ast.ASTHandle) (int32, error) {
	m, e := c.Manager()
	if e != nil {
		return 0, e
	}
	return m.GetDeclKind(d)
}
func (c *SMTContext) VariableIndex(a ast.ASTHandle) (uint32, error) {
	m, e := c.Manager()
	if e != nil {
		return 0, e
	}
	return m.GetVarIndex(a)
}
func (c *SMTContext) BooleanValue(a ast.ASTHandle) (lbool.LBool, error) {
	m, e := c.Manager()
	if e != nil {
		return lbool.Undef, e
	}
	if _, e = m.GetSort(a); e != nil {
		return lbool.Undef, e
	}
	if m.IsTrue(a) {
		return lbool.True, nil
	}
	if m.IsFalse(a) {
		return lbool.False, nil
	}
	return lbool.Undef, nil
}
