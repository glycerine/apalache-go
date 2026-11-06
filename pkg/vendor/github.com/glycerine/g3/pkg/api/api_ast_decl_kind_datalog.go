package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind datalog-family switch;
// src/ast/dl_decl_plugin.h dl_op_kind.

import "github.com/glycerine/g3/pkg/ast"

const (
	APIDeclarationRelationStore APIDeclarationKind = 520 + iota
	APIDeclarationRelationEmpty
	APIDeclarationRelationIsEmpty
	APIDeclarationRelationJoin
	APIDeclarationRelationUnion
	APIDeclarationRelationWiden
	APIDeclarationRelationProject
	APIDeclarationRelationFilter
	APIDeclarationRelationNegationFilter
	APIDeclarationRelationRename
	APIDeclarationRelationComplement
	APIDeclarationRelationSelect
	APIDeclarationRelationClone
	APIDeclarationFiniteDomainConstant
	APIDeclarationFiniteDomainLessThan
)

// DatalogDeclarationKind classifies a retained relational-algebra or
// finite-domain declaration/application.
func (c *SMTContext) DatalogDeclarationKind(value ast.ASTHandle) (APIDeclarationKind, error) {
	manager, family, operation, err := c.apiDeclarationFamilyOperation(value)
	if err != nil {
		return APIDeclarationInternal, err
	}
	datalog, err := manager.RegisterDatalogPlugin()
	if err != nil {
		return APIDeclarationInternal, err
	}
	if family != datalog {
		return APIDeclarationInternal, ast.ErrASTDomain
	}
	return apiDatalogDeclarationKind(operation), nil
}

func apiDatalogDeclarationKind(operation int32) APIDeclarationKind {
	if kind := apiDatalogRelationDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiDatalogFilterDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	return apiDatalogFiniteDeclarationKind(operation)
}
