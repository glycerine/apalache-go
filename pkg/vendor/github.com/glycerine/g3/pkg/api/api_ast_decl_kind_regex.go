package api

// Source regular-expression cases retained by Z3_get_decl_kind. OP_RE_RANGE,
// OP_RE_COMPLEMENT, and OP_RE_FULL_SEQ_SET are deliberately not public there.
func apiRegexDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 13:
		return APIDeclarationRegexPlus
	case 14:
		return APIDeclarationRegexStar
	case 15:
		return APIDeclarationRegexOption
	case 17:
		return APIDeclarationRegexConcat
	case 18:
		return APIDeclarationRegexUnion
	case 19:
		return APIDeclarationRegexIntersect
	case 20:
		return APIDeclarationRegexLoop
	case 22:
		return APIDeclarationRegexEmptySet
	case 24:
		return APIDeclarationRegexFullSet
	default:
		return APIDeclarationInternal
	}
}
