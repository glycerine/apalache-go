package api

func apiDatalogFiniteDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 13:
		return APIDeclarationFiniteDomainConstant
	case 14:
		return APIDeclarationFiniteDomainLessThan
	default:
		return APIDeclarationInternal
	}
}
