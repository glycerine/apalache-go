package api

// Source: src/api/api_ast.cpp:377-385 and 549-557 get_decl_name and
// get_sort_name. These methods retain the symbol kind instead of flattening
// numerical symbols to strings.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *SMTContext) typedASTSymbol(value ast.ASTHandle, required ast.ASTKind) (usymbol.Symbol, error) {
	manager, err := c.Manager()
	if err != nil {
		return usymbol.Symbol{}, err
	}
	kind, err := manager.GetKind(value)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	if kind != required {
		return usymbol.Symbol{}, ast.ErrASTDomain
	}
	return manager.GetASTName(value)
}

// DeclarationSymbol returns the declaration's native symbol, including a
// numerical symbol without converting it to decimal text.
func (c *SMTContext) DeclarationSymbol(declaration ast.ASTHandle) (usymbol.Symbol, error) {
	return c.typedASTSymbol(declaration, ast.ASTFuncDecl)
}

// SortSymbol returns the sort's native symbol.
func (c *SMTContext) SortSymbol(sort ast.ASTHandle) (usymbol.Symbol, error) {
	return c.typedASTSymbol(sort, ast.ASTSort)
}
