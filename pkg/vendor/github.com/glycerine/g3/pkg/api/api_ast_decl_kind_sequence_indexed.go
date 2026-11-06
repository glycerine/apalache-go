package api

// Source sequence extraction, replacement, indexing, and length operations.
func apiSequenceIndexedDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 6:
		return APIDeclarationSequenceExtract
	case 7:
		return APIDeclarationSequenceReplace
	case 8:
		return APIDeclarationSequenceAt
	case 9:
		return APIDeclarationSequenceLength
	case 10:
		return APIDeclarationSequenceIndex
	default:
		return APIDeclarationInternal
	}
}
