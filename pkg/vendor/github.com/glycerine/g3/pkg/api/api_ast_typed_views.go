package api

// Source: src/api/api_ast.cpp:225-240 and 341-355. Native Z3 exposes
// distinct app, sort, function-declaration, and AST pointer views. Go keeps a
// single handle representation but preserves the checked view conversions.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) typedASTView(value ast.ASTHandle, required ast.ASTKind) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := manager.GetKind(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind != required {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return value, nil
}

// ApplicationToAST returns the general AST view of an application.
func (c *SMTContext) ApplicationToAST(application ast.ASTHandle) (ast.ASTHandle, error) {
	return c.typedASTView(application, ast.ASTApp)
}

// SortToAST returns the general AST view of a sort.
func (c *SMTContext) SortToAST(sort ast.ASTHandle) (ast.ASTHandle, error) {
	return c.typedASTView(sort, ast.ASTSort)
}

// FunctionDeclarationToAST returns the general AST view of a declaration.
func (c *SMTContext) FunctionDeclarationToAST(declaration ast.ASTHandle) (ast.ASTHandle, error) {
	return c.typedASTView(declaration, ast.ASTFuncDecl)
}

// ToApplication checks and returns the application view of an AST.
func (c *SMTContext) ToApplication(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.typedASTView(value, ast.ASTApp)
}

// ToSort checks and returns the sort view of an AST.
func (c *SMTContext) ToSort(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.typedASTView(value, ast.ASTSort)
}

// ToFunctionDeclaration checks and returns the declaration view of an AST.
func (c *SMTContext) ToFunctionDeclaration(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.typedASTView(value, ast.ASTFuncDecl)
}
