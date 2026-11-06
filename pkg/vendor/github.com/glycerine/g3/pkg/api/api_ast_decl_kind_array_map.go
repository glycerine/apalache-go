package api

import "github.com/glycerine/g3/pkg/ast"

func apiArrayDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case ast.ASTArrayOpStore:
		return APIDeclarationArrayStore
	case ast.ASTArrayOpSelect:
		return APIDeclarationArraySelect
	case ast.ASTArrayOpConst:
		return APIDeclarationConstArray
	case ast.ASTArrayOpDefault:
		return APIDeclarationArrayDefault
	case ast.ASTArrayOpMap:
		return APIDeclarationArrayMap
	case ast.ASTArrayOpSetUnion:
		return APIDeclarationSetUnion
	case ast.ASTArrayOpSetIntersect:
		return APIDeclarationSetIntersect
	case ast.ASTArrayOpSetDifference:
		return APIDeclarationSetDifference
	case ast.ASTArrayOpSetComplement:
		return APIDeclarationSetComplement
	case ast.ASTArrayOpSetSubset:
		return APIDeclarationSetSubset
	case ast.ASTArrayOpAsArray:
		return APIDeclarationAsArray
	case ast.ASTArrayOpExt:
		return APIDeclarationArrayExtensionality
	default:
		return APIDeclarationInternal
	}
}
