package api

func apiDatalogFilterDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 7:
		return APIDeclarationRelationFilter
	case 8:
		return APIDeclarationRelationNegationFilter
	case 9:
		return APIDeclarationRelationRename
	case 10:
		return APIDeclarationRelationComplement
	case 11:
		return APIDeclarationRelationSelect
	case 12:
		return APIDeclarationRelationClone
	default:
		return APIDeclarationInternal
	}
}
