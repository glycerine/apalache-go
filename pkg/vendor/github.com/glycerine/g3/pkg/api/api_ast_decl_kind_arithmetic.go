package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind arithmetic family switch.

import "github.com/glycerine/g3/pkg/ast"

const (
	APIDeclarationArithmeticNumeral APIDeclarationKind = 200 + iota
	APIDeclarationAlgebraicNumeral
	APIDeclarationLE
	APIDeclarationGE
	APIDeclarationLT
	APIDeclarationGT
	APIDeclarationAdd
	APIDeclarationSubtract
	APIDeclarationUnaryMinus
	APIDeclarationMultiply
	APIDeclarationDivide
	APIDeclarationIntegerDivide
	APIDeclarationRemainder
	APIDeclarationModulo
	APIDeclarationPower
	APIDeclarationToReal
	APIDeclarationToInteger
	APIDeclarationIsInteger
)

// ArithmeticDeclarationKind classifies an arithmetic declaration or app.
func (c *SMTContext) ArithmeticDeclarationKind(value ast.ASTHandle) (APIDeclarationKind, error) {
	manager, family, operation, err := c.apiDeclarationFamilyOperation(value)
	if err != nil {
		return APIDeclarationInternal, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return APIDeclarationInternal, err
	}
	if family != arithmetic {
		return APIDeclarationInternal, ast.ErrASTDomain
	}
	return apiArithmeticDeclarationKind(operation), nil
}
