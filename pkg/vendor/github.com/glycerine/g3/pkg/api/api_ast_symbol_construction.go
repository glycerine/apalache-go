package api

// Source: src/api/api_ast.cpp:78-138,463-478. Source names are symbols, which
// may be numerical or null; the string convenience constructors cannot express
// every such value without these typed entry points.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *SMTContext) UninterpretedSortWithSymbol(name usymbol.Symbol) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkUninterpretedSort(name, nil)
}

func (c *SMTContext) FunctionWithSymbol(name usymbol.Symbol, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkFuncDecl(name, domain, rangeSort, ast.ASTFuncDeclInfo{})
}

func (c *SMTContext) ConstantWithSymbol(name usymbol.Symbol, sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkConst(name, sort)
}

// DeclarationParameterSymbol preserves the source symbol kind and identity.
// The returned symbol borrows this context's symbol manager.
func (c *SMTContext) DeclarationParameterSymbol(declaration ast.ASTHandle, index uint32) (result usymbol.Symbol, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return usymbol.Symbol{}, err
	}
	parameter, err := apiDeclarationParameter(m, declaration, index)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	return m.ParameterSymbol(parameter)
}
