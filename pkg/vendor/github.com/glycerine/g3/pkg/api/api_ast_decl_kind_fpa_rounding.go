package api

func apiFPARoundingDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 0:
		return APIDeclarationFPARMNearestTiesToEven
	case 1:
		return APIDeclarationFPARMNearestTiesToAway
	case 2:
		return APIDeclarationFPARMTowardPositive
	case 3:
		return APIDeclarationFPARMTowardNegative
	case 4:
		return APIDeclarationFPARMTowardZero
	default:
		return APIDeclarationInternal
	}
}
