package api

// Source: src/api/api_ast.cpp:880-1210, Z3_get_decl_kind.

import "github.com/glycerine/g3/pkg/ast"

// PublicDeclarationKind classifies every source theory family into the public
// API categories. Applications are accepted as a convenience declaration view.
// DeclarationKind continues to expose the manager's family-local operation.
func (c *SMTContext) PublicDeclarationKind(value ast.ASTHandle) (APIDeclarationKind, error) {
	m, family, operation, err := c.apiDeclarationFamilyOperation(value)
	if err != nil {
		return APIDeclarationUninterpreted, err
	}
	if family == ast.NullFamilyID {
		return APIDeclarationUninterpreted, nil
	}
	if family == m.BasicFamilyID() {
		return apiBasicDeclarationKind(operation), nil
	}
	if family == ast.FamilyID(1) {
		return apiLabelDeclarationKind(operation), nil
	}
	name, err := apiRegisteredFamilyName(m, family)
	if err != nil {
		return APIDeclarationUninterpreted, err
	}
	switch name {
	case "arith":
		return apiArithmeticDeclarationKind(operation), nil
	case "array":
		return apiArrayDeclarationKind(operation), nil
	case "bv":
		return apiBitVectorDeclarationKind(operation), nil
	case "datatype":
		return apiDatatypeDeclarationKind(operation), nil
	case "datalog_relation":
		return apiDatalogDeclarationKind(operation), nil
	case "seq":
		return apiSequenceDeclarationKind(operation), nil
	case "fpa":
		return apiFloatingPointDeclarationKind(operation), nil
	case "pb":
		return apiPseudoBooleanDeclarationKind(operation), nil
	default:
		return APIDeclarationUninterpreted, nil
	}
}
