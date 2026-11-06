package api

// Source: src/api/api_ast.cpp:73-96,225-240,567-581.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) SortEqual(left, right ast.ASTHandle) (bool, error) {
	m, err := c.Manager()
	if err != nil {
		return false, err
	}
	for _, value := range []ast.ASTHandle{left, right} {
		if kind, e := m.GetKind(value); e != nil || kind != ast.ASTSort {
			if e != nil {
				return false, e
			}
			return false, ast.ErrASTDomain
		}
	}
	return left == right, nil
}

func (c *SMTContext) FunctionEqual(left, right ast.ASTHandle) (bool, error) {
	m, err := c.Manager()
	if err != nil {
		return false, err
	}
	for _, value := range []ast.ASTHandle{left, right} {
		if kind, e := m.GetKind(value); e != nil || kind != ast.ASTFuncDecl {
			if e != nil {
				return false, e
			}
			return false, ast.ErrASTDomain
		}
	}
	return left == right, nil
}

func (c *SMTContext) DeclarationDomainSize(declaration ast.ASTHandle) (uint32, error) {
	m, err := c.Manager()
	if err != nil {
		return 0, err
	}
	return m.GetArity(declaration)
}

func (c *SMTContext) ApplicationAST(application ast.ASTHandle) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind, err := m.GetKind(application); err != nil || kind != ast.ASTApp {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return application, nil
}
