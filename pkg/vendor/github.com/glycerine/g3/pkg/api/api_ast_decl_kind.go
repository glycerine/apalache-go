package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind. The source public operation
// enumeration is distinct from each plugin's private integer operation code.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// APIDeclarationKind is the public classification produced by Z3_get_decl_kind.
// Each theory's source operation values map to distinct categories below.
type APIDeclarationKind uint16

const (
	APIDeclarationUninterpreted APIDeclarationKind = iota
	APIDeclarationInternal
	APIDeclarationTrue
	APIDeclarationFalse
	APIDeclarationEqual
	APIDeclarationDistinct
	APIDeclarationIte
	APIDeclarationAnd
	APIDeclarationOr
	APIDeclarationIff
	APIDeclarationXor
	APIDeclarationNot
	APIDeclarationImplies
	APIDeclarationOEq
	APIDeclarationInterpolant
	APIDeclarationPBLE
	APIDeclarationPBGE
	APIDeclarationPBEQ
	APIDeclarationPBAtMost
	APIDeclarationPBAtLeast
)

// CoreDeclarationKind is the basic, proof, and pseudo-Boolean specialization.
// PublicDeclarationKind dispatches across every source family.
func (c *SMTContext) CoreDeclarationKind(declaration ast.ASTHandle) (APIDeclarationKind, error) {
	manager, err := c.Manager()
	if err != nil {
		return APIDeclarationUninterpreted, err
	}
	kind, err := manager.GetKind(declaration)
	if err != nil {
		return APIDeclarationUninterpreted, err
	}
	if kind == ast.ASTApp {
		declaration, err = manager.GetDecl(declaration)
		if err != nil {
			return APIDeclarationUninterpreted, err
		}
	} else if kind != ast.ASTFuncDecl {
		return APIDeclarationUninterpreted, ast.ErrASTDomain
	}
	family, err := manager.GetFamilyID(declaration)
	if err != nil {
		return APIDeclarationUninterpreted, err
	}
	operation, err := manager.GetDeclKind(declaration)
	if err != nil {
		return APIDeclarationUninterpreted, err
	}
	if family < 0 {
		return APIDeclarationUninterpreted, nil
	}
	if family == manager.BasicFamilyID() {
		return apiBasicDeclarationKind(operation), nil
	}
	familyName, err := manager.FamilyName(family)
	if err != nil {
		return APIDeclarationUninterpreted, err
	}
	familyText, err := manager.Symbols.Display(familyName)
	if err != nil {
		return APIDeclarationUninterpreted, err
	}
	if familyText == "pb" {
		return apiPseudoBooleanDeclarationKind(operation), nil
	}
	return APIDeclarationInternal, ast.ErrASTDomain
}
