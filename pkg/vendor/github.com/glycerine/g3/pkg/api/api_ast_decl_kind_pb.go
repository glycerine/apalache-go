package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind pseudo-Boolean operation switch.

import "github.com/glycerine/g3/pkg/ast"

func apiPseudoBooleanDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case ast.ASTPBOpLE:
		return APIDeclarationPBLE
	case ast.ASTPBOpGE:
		return APIDeclarationPBGE
	case ast.ASTPBOpEQ:
		return APIDeclarationPBEQ
	case ast.ASTPBOpAtMost:
		return APIDeclarationPBAtMost
	case ast.ASTPBOpAtLeast:
		return APIDeclarationPBAtLeast
	default:
		return APIDeclarationInternal
	}
}
