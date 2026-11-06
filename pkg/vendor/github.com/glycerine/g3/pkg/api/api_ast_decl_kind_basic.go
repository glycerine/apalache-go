package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind basic operation switch.

import "github.com/glycerine/g3/pkg/ast"

func apiBasicDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case ast.ASTOpTrue:
		return APIDeclarationTrue
	case ast.ASTOpFalse:
		return APIDeclarationFalse
	case ast.ASTOpEq:
		return APIDeclarationEqual
	case ast.ASTOpDistinct:
		return APIDeclarationDistinct
	case ast.ASTOpIte:
		return APIDeclarationIte
	case ast.ASTOpAnd:
		return APIDeclarationAnd
	case ast.ASTOpOr:
		return APIDeclarationOr
	case ast.ASTOpIff:
		return APIDeclarationIff
	case ast.ASTOpXor:
		return APIDeclarationXor
	case ast.ASTOpNot:
		return APIDeclarationNot
	case ast.ASTOpImplies:
		return APIDeclarationImplies
	case ast.ASTOpOEq:
		return APIDeclarationOEq
	case ast.ASTOpInterp:
		return APIDeclarationInterpolant
	default:
		return apiProofDeclarationKind(operation)
	}
}
