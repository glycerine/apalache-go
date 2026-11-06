package api

// Source: src/api/api_ast.cpp:819-851 ast_to_string, sort_to_string, and
// func_decl_to_string. Print mode is explicit in Go through APIASTPrinter.

import "github.com/glycerine/g3/pkg/ast"

// ASTString renders any valid AST using the selected source print mode.
func (c *SMTContext) ASTString(value ast.ASTHandle, mode APIASTPrintMode) (string, error) {
	manager, err := c.Manager()
	if err != nil {
		return "", err
	}
	if _, err = manager.GetKind(value); err != nil {
		return "", err
	}
	printer, err := c.NewASTPrinter(mode)
	if err != nil {
		return "", err
	}
	return printer.String(value)
}

// SortString checks the sort view before rendering it.
func (c *SMTContext) SortString(sort ast.ASTHandle, mode APIASTPrintMode) (string, error) {
	if _, err := c.ToSort(sort); err != nil {
		return "", err
	}
	return c.ASTString(sort, mode)
}

// FunctionString checks the function-declaration view before rendering it.
func (c *SMTContext) FunctionString(declaration ast.ASTHandle, mode APIASTPrintMode) (string, error) {
	if _, err := c.ToFunctionDeclaration(declaration); err != nil {
		return "", err
	}
	return c.ASTString(declaration, mode)
}
