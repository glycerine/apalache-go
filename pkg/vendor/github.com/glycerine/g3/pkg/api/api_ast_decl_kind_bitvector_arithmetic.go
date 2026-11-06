package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind bit-vector arithmetic cases.

func apiBitVectorArithmeticDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 0:
		return APIDeclarationBitVectorNumeral
	case 1:
		return APIDeclarationBitOne
	case 2:
		return APIDeclarationBitZero
	case 3:
		return APIDeclarationBitVectorNegate
	case 4:
		return APIDeclarationBitVectorAdd
	case 5:
		return APIDeclarationBitVectorSubtract
	case 6:
		return APIDeclarationBitVectorMultiply
	case 7:
		return APIDeclarationBitVectorSignedDivide
	case 8:
		return APIDeclarationBitVectorUnsignedDivide
	case 9:
		return APIDeclarationBitVectorSignedRemainder
	case 10:
		return APIDeclarationBitVectorUnsignedRemainder
	case 11:
		return APIDeclarationBitVectorSignedModulo
	case 12:
		return APIDeclarationBitVectorSignedDivideByZero
	case 13:
		return APIDeclarationBitVectorUnsignedDivideByZero
	case 14, 15:
		// The source deliberately reports both signed and unsigned rem0 as
		// Z3_OP_BUREM0; there is no distinct public signed-rem0 kind.
		return APIDeclarationBitVectorUnsignedRemainderByZero
	case 16:
		return APIDeclarationBitVectorSignedModuloByZero
	case 17:
		return APIDeclarationBitVectorSignedDivideInternal
	case 18:
		return APIDeclarationBitVectorUnsignedDivideInternal
	case 19:
		return APIDeclarationBitVectorSignedRemainderInternal
	case 20:
		return APIDeclarationBitVectorUnsignedRemainderInternal
	case 21:
		return APIDeclarationBitVectorSignedModuloInternal
	default:
		return APIDeclarationInternal
	}
}
