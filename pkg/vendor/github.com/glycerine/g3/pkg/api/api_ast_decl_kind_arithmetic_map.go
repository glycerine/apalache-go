package api

// Arithmetic operation numbers are the exact arith_decl_plugin enumeration
// consumed by src/api/api_ast.cpp.
func apiArithmeticDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 0:
		return APIDeclarationArithmeticNumeral
	case 1:
		return APIDeclarationAlgebraicNumeral
	case 2:
		return APIDeclarationLE
	case 3:
		return APIDeclarationGE
	case 4:
		return APIDeclarationLT
	case 5:
		return APIDeclarationGT
	case 6:
		return APIDeclarationAdd
	case 7:
		return APIDeclarationSubtract
	case 8:
		return APIDeclarationUnaryMinus
	case 9:
		return APIDeclarationMultiply
	case 10:
		return APIDeclarationDivide
	case 11:
		return APIDeclarationIntegerDivide
	case 12:
		return APIDeclarationRemainder
	case 13:
		return APIDeclarationModulo
	case 18:
		return APIDeclarationPower
	case 14:
		return APIDeclarationToReal
	case 15:
		return APIDeclarationToInteger
	case 16:
		return APIDeclarationIsInteger
	default:
		return APIDeclarationInternal
	}
}
