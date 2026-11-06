package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind bit-vector overflow and conversion cases.

func apiBitVectorOverflowDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 52:
		return APIDeclarationBitVectorUnsignedMultiplyNoOverflow
	case 53:
		return APIDeclarationBitVectorSignedMultiplyNoOverflow
	case 54:
		return APIDeclarationBitVectorSignedMultiplyNoUnderflow
	case 55:
		return APIDeclarationBitToBoolean
	case 57:
		return APIDeclarationIntegerToBitVector
	case 58:
		return APIDeclarationBitVectorToInteger
	case 59:
		return APIDeclarationBitVectorCarry
	case 60:
		return APIDeclarationBitVectorXor3
	default:
		return APIDeclarationInternal
	}
}

func apiBitVectorDeclarationKind(operation int32) APIDeclarationKind {
	if kind := apiBitVectorArithmeticDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiBitVectorRelationDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiBitVectorStructuralDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	return apiBitVectorOverflowDeclarationKind(operation)
}
