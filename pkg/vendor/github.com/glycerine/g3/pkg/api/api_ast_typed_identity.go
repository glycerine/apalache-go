package api

// Source: src/api/api_ast.cpp:242-263 get_func_decl_id and get_sort_id.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) typedASTID(value ast.ASTHandle, required ast.ASTKind) (uint32, error) {
	manager, err := c.Manager()
	if err != nil {
		return 0, err
	}
	kind, err := manager.GetKind(value)
	if err != nil {
		return 0, err
	}
	if kind != required {
		return 0, ast.ErrASTDomain
	}
	return manager.GetASTID(value)
}

// FunctionDeclarationID returns the manager-local source AST identifier.
func (c *SMTContext) FunctionDeclarationID(declaration ast.ASTHandle) (uint32, error) {
	return c.typedASTID(declaration, ast.ASTFuncDecl)
}

// SortID returns the manager-local source AST identifier.
func (c *SMTContext) SortID(sort ast.ASTHandle) (uint32, error) {
	return c.typedASTID(sort, ast.ASTSort)
}
