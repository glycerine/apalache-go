package api

func apiFPAArithmeticDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 11:
		return APIDeclarationFPAAdd
	case 12:
		return APIDeclarationFPASubtract
	case 13:
		return APIDeclarationFPANegate
	case 14:
		return APIDeclarationFPAMultiply
	case 15:
		return APIDeclarationFPADivide
	case 16:
		return APIDeclarationFPARemainder
	case 17:
		return APIDeclarationFPAAbsolute
	case 18:
		return APIDeclarationFPAMinimum
	case 19:
		return APIDeclarationFPAMaximum
	case 20:
		return APIDeclarationFPAFusedMultiplyAdd
	case 21:
		return APIDeclarationFPASquareRoot
	case 22:
		return APIDeclarationFPARoundToIntegral
	default:
		return APIDeclarationInternal
	}
}
