package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind label-family switch;
// src/ast/ast.h label_op_kind. The label family is source family ID 1.

import "github.com/glycerine/g3/pkg/ast"

const (
	APIDeclarationLabel APIDeclarationKind = 540 + iota
	APIDeclarationLabelLiteral
)

func apiLabelDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 0:
		return APIDeclarationLabel
	case 1:
		return APIDeclarationLabelLiteral
	default:
		return APIDeclarationInternal
	}
}

// LabelDeclarationKind classifies the two concrete label-family operations.
func (c *SMTContext) LabelDeclarationKind(value ast.ASTHandle) (APIDeclarationKind, error) {
	_, family, operation, err := c.apiDeclarationFamilyOperation(value)
	if err != nil {
		return APIDeclarationInternal, err
	}
	if family != ast.FamilyID(1) {
		return APIDeclarationInternal, ast.ErrASTDomain
	}
	return apiLabelDeclarationKind(operation), nil
}
