package api

// Source operations OP_SEQ_PREFIX, OP_SEQ_SUFFIX, and OP_SEQ_CONTAINS.
func apiSequenceRelationDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 3:
		return APIDeclarationSequencePrefix
	case 4:
		return APIDeclarationSequenceSuffix
	case 5:
		return APIDeclarationSequenceContains
	default:
		return APIDeclarationInternal
	}
}
