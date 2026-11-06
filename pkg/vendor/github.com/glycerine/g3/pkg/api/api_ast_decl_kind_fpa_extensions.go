package api

func apiFPAExtensionDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 42:
		return APIDeclarationFPABitVectorWrap
	case 43:
		return APIDeclarationFPABitVectorToRoundingMode
	default:
		return APIDeclarationInternal
	}
}
