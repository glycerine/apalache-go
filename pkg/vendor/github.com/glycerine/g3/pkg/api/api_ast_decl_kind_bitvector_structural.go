package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind bit-vector structural cases.

func apiBitVectorStructuralDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 37:
		return APIDeclarationBitVectorConcat
	case 38:
		return APIDeclarationBitVectorSignExtend
	case 39:
		return APIDeclarationBitVectorZeroExtend
	case 40:
		return APIDeclarationBitVectorExtract
	case 41:
		return APIDeclarationBitVectorRepeat
	case 42:
		return APIDeclarationBitVectorReduceOr
	case 43:
		return APIDeclarationBitVectorReduceAnd
	case 44:
		return APIDeclarationBitVectorCompare
	case 45:
		return APIDeclarationBitVectorShiftLeft
	case 46:
		return APIDeclarationBitVectorLogicalShiftRight
	case 47:
		return APIDeclarationBitVectorArithmeticShiftRight
	case 48:
		return APIDeclarationBitVectorRotateLeft
	case 49:
		return APIDeclarationBitVectorRotateRight
	case 50:
		return APIDeclarationBitVectorExtendedRotateLeft
	case 51:
		return APIDeclarationBitVectorExtendedRotateRight
	default:
		return APIDeclarationInternal
	}
}
