package api

func apiFPAValueDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 5:
		return APIDeclarationFPANumeral
	case 6:
		return APIDeclarationFPAPlusInfinity
	case 7:
		return APIDeclarationFPAMinusInfinity
	case 8:
		return APIDeclarationFPANaN
	case 9:
		return APIDeclarationFPAPlusZero
	case 10:
		return APIDeclarationFPAMinusZero
	default:
		return APIDeclarationInternal
	}
}
