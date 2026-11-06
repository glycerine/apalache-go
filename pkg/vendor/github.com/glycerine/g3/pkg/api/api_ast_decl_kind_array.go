package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind array family switch.

import "github.com/glycerine/g3/pkg/ast"

const (
	APIDeclarationArrayStore APIDeclarationKind = 240 + iota
	APIDeclarationArraySelect
	APIDeclarationConstArray
	APIDeclarationArrayDefault
	APIDeclarationArrayMap
	APIDeclarationSetUnion
	APIDeclarationSetIntersect
	APIDeclarationSetDifference
	APIDeclarationSetComplement
	APIDeclarationSetSubset
	APIDeclarationAsArray
	APIDeclarationArrayExtensionality
)

// ArrayDeclarationKind classifies an array declaration or application.
func (c *SMTContext) ArrayDeclarationKind(value ast.ASTHandle) (APIDeclarationKind, error) {
	manager, family, operation, err := c.apiDeclarationFamilyOperation(value)
	if err != nil {
		return APIDeclarationInternal, err
	}
	array, err := manager.ArrayFamilyID()
	if err != nil {
		return APIDeclarationInternal, err
	}
	if family != array {
		return APIDeclarationInternal, ast.ErrASTDomain
	}
	return apiArrayDeclarationKind(operation), nil
}
