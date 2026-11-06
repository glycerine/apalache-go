package api

// Source: src/api/api_ast.cpp:391-548.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// APIDeclarationParameterKind is the public classification used by the native
// declaration-parameter inspection API. AST-backed parameters are refined by
// the kind of node they contain.
type APIDeclarationParameterKind uint8

const (
	APIDeclarationParameterInteger APIDeclarationParameterKind = iota
	APIDeclarationParameterDouble
	APIDeclarationParameterRational
	APIDeclarationParameterSymbol
	APIDeclarationParameterSort
	APIDeclarationParameterAST
	APIDeclarationParameterFunctionDeclaration
)

func apiDeclarationParameter(m *ast.ASTManager, declaration ast.ASTHandle, index uint32) (selected ast.ASTParameter, resultErr error) {
	return m.GetDeclParameterCopy(declaration, index)
}

func (c *SMTContext) DeclarationParameterCount(declaration ast.ASTHandle) (uint32, error) {
	m, err := c.Manager()
	if err != nil {
		return 0, err
	}
	return m.GetDeclNumParameters(declaration)
}

func (c *SMTContext) DeclarationParameterKind(declaration ast.ASTHandle, index uint32) (result APIDeclarationParameterKind, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return 0, err
	}
	parameter, err := apiDeclarationParameter(m, declaration, index)
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	kind, err := m.ParameterKind(parameter)
	if err != nil {
		return 0, err
	}
	switch kind {
	case ast.ASTParameterInt:
		return APIDeclarationParameterInteger, nil
	case ast.ASTParameterDouble:
		return APIDeclarationParameterDouble, nil
	case ast.ASTParameterRational:
		return APIDeclarationParameterRational, nil
	case ast.ASTParameterSymbol:
		return APIDeclarationParameterSymbol, nil
	case ast.ASTParameterAST:
		value, valueErr := m.ParameterAST(parameter)
		if valueErr != nil {
			return 0, valueErr
		}
		nodeKind, valueErr := m.GetKind(value)
		if valueErr != nil {
			return 0, valueErr
		}
		switch nodeKind {
		case ast.ASTSort:
			return APIDeclarationParameterSort, nil
		case ast.ASTFuncDecl:
			return APIDeclarationParameterFunctionDeclaration, nil
		default:
			return APIDeclarationParameterAST, nil
		}
	default:
		return 0, ast.ErrASTDomain
	}
}

func (c *SMTContext) DeclarationIntegerParameter(declaration ast.ASTHandle, index uint32) (result int32, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return 0, err
	}
	parameter, err := apiDeclarationParameter(m, declaration, index)
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	return m.ParameterInt(parameter)
}

func (c *SMTContext) DeclarationDoubleParameter(declaration ast.ASTHandle, index uint32) (result float64, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return 0, err
	}
	parameter, err := apiDeclarationParameter(m, declaration, index)
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	return m.ParameterDouble(parameter)
}

func (c *SMTContext) DeclarationSymbolParameter(declaration ast.ASTHandle, index uint32) (result string, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return "", err
	}
	parameter, err := apiDeclarationParameter(m, declaration, index)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	symbol, err := m.ParameterSymbol(parameter)
	if err != nil {
		return "", err
	}
	return m.Symbols.Display(symbol)
}

func (c *SMTContext) DeclarationRationalParameter(declaration ast.ASTHandle, index uint32) (result string, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return "", err
	}
	parameter, err := apiDeclarationParameter(m, declaration, index)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	service := m.NumeralService()
	if service == nil {
		return "", ast.ErrASTHandle
	}
	value, err := service.NewValue()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	matched, err := m.ParameterRationalCopy(parameter, &value)
	if err != nil {
		return "", err
	}
	if !matched {
		return "", ast.ErrASTDomain
	}
	return service.String(&value)
}

func (c *SMTContext) declarationASTParameter(declaration ast.ASTHandle, index uint32, required *ast.ASTKind) (result ast.ASTHandle, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parameter, err := apiDeclarationParameter(m, declaration, index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	result, err = m.ParameterAST(parameter)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if required != nil {
		kind, kindErr := m.GetKind(result)
		if kindErr != nil {
			return ast.ASTHandle{}, kindErr
		}
		if kind != *required {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	return result, nil
}

func (c *SMTContext) DeclarationASTParameter(declaration ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	return c.declarationASTParameter(declaration, index, nil)
}

func (c *SMTContext) DeclarationSortParameter(declaration ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	kind := ast.ASTSort
	return c.declarationASTParameter(declaration, index, &kind)
}

func (c *SMTContext) DeclarationFunctionParameter(declaration ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	kind := ast.ASTFuncDecl
	return c.declarationASTParameter(declaration, index, &kind)
}
