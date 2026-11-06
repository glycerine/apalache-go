package api

// Source operations OP_SEQ_UNIT, OP_SEQ_EMPTY, OP_SEQ_CONCAT, OP_SEQ_TO_RE,
// and OP_SEQ_IN_RE.
func apiSequenceCoreDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 0:
		return APIDeclarationSequenceUnit
	case 1:
		return APIDeclarationSequenceEmpty
	case 2:
		return APIDeclarationSequenceConcat
	case 11:
		return APIDeclarationSequenceToRegex
	case 12:
		return APIDeclarationSequenceInRegex
	default:
		return APIDeclarationInternal
	}
}
