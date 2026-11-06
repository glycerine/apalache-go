package api

func apiDatalogRelationDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 0:
		return APIDeclarationRelationStore
	case 1:
		return APIDeclarationRelationEmpty
	case 2:
		return APIDeclarationRelationIsEmpty
	case 3:
		return APIDeclarationRelationJoin
	case 4:
		return APIDeclarationRelationUnion
	case 5:
		return APIDeclarationRelationWiden
	case 6:
		return APIDeclarationRelationProject
	default:
		return APIDeclarationInternal
	}
}
