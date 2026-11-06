package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind bit-vector relation and logic cases.

func apiBitVectorRelationDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 22:
		return APIDeclarationBitVectorUnsignedLE
	case 23:
		return APIDeclarationBitVectorSignedLE
	case 24:
		return APIDeclarationBitVectorUnsignedGE
	case 25:
		return APIDeclarationBitVectorSignedGE
	case 26:
		return APIDeclarationBitVectorUnsignedLT
	case 27:
		return APIDeclarationBitVectorSignedLT
	case 28:
		return APIDeclarationBitVectorUnsignedGT
	case 29:
		return APIDeclarationBitVectorSignedGT
	case 30:
		return APIDeclarationBitVectorAnd
	case 31:
		return APIDeclarationBitVectorOr
	case 32:
		return APIDeclarationBitVectorNot
	case 33:
		return APIDeclarationBitVectorXor
	case 34:
		return APIDeclarationBitVectorNand
	case 35:
		return APIDeclarationBitVectorNor
	case 36:
		return APIDeclarationBitVectorXnor
	default:
		return APIDeclarationInternal
	}
}
