package api

func apiFPAConversionDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 35:
		return APIDeclarationFPAConstruct
	case 36:
		return APIDeclarationFPAToFloatingPoint
	case 37:
		return APIDeclarationFPAToFloatingPointUnsigned
	case 38:
		return APIDeclarationFPAToUnsignedBitVector
	case 39:
		return APIDeclarationFPAToSignedBitVector
	case 40:
		return APIDeclarationFPAToReal
	case 41:
		return APIDeclarationFPAToIEEEBitVector
	default:
		return APIDeclarationInternal
	}
}
