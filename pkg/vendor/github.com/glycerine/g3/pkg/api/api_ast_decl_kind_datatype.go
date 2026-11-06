package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind datatype family switch.

import "github.com/glycerine/g3/pkg/ast"

const (
	APIDeclarationDatatypeConstructor APIDeclarationKind = 380 + iota
	APIDeclarationDatatypeRecognizer
	APIDeclarationDatatypeIs
	APIDeclarationDatatypeAccessor
	APIDeclarationDatatypeUpdateField
)

func apiDatatypeDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case ast.ASTDatatypeConstructor:
		return APIDeclarationDatatypeConstructor
	case ast.ASTDatatypeRecognizer:
		return APIDeclarationDatatypeRecognizer
	case ast.ASTDatatypeIs:
		return APIDeclarationDatatypeIs
	case ast.ASTDatatypeAccessor:
		return APIDeclarationDatatypeAccessor
	case ast.ASTDatatypeUpdateField:
		return APIDeclarationDatatypeUpdateField
	default:
		return APIDeclarationInternal
	}
}

// DatatypeDeclarationKind classifies a datatype declaration or app.
func (c *SMTContext) DatatypeDeclarationKind(value ast.ASTHandle) (APIDeclarationKind, error) {
	manager, family, operation, err := c.apiDeclarationFamilyOperation(value)
	if err != nil {
		return APIDeclarationInternal, err
	}
	datatype, err := manager.DatatypeFamilyID()
	if err != nil {
		return APIDeclarationInternal, err
	}
	if family != datatype {
		return APIDeclarationInternal, ast.ErrASTDomain
	}
	return apiDatatypeDeclarationKind(operation), nil
}
