package api

func apiFPARelationDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 23:
		return APIDeclarationFPAEqual
	case 24:
		return APIDeclarationFPALessThan
	case 25:
		return APIDeclarationFPAGreaterThan
	case 26:
		return APIDeclarationFPALessEqual
	case 27:
		return APIDeclarationFPAGreaterEqual
	case 28:
		return APIDeclarationFPAIsNaN
	case 29:
		return APIDeclarationFPAIsInfinite
	case 30:
		return APIDeclarationFPAIsZero
	case 31:
		return APIDeclarationFPAIsNormal
	case 32:
		return APIDeclarationFPAIsSubnormal
	case 33:
		return APIDeclarationFPAIsNegative
	case 34:
		return APIDeclarationFPAIsPositive
	default:
		return APIDeclarationInternal
	}
}
