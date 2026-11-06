package api

// Source string conversions and legacy string-operation aliases. The aliases
// intentionally report their corresponding public sequence operations.
func apiSequenceStringDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case 27:
		return APIDeclarationIntegerToString
	case 28:
		return APIDeclarationStringToInteger
	case 29:
		return APIDeclarationSequenceReplace
	case 30:
		return APIDeclarationSequenceConcat
	case 31:
		return APIDeclarationSequenceLength
	case 32:
		return APIDeclarationSequenceContains
	case 33:
		return APIDeclarationSequencePrefix
	case 34:
		return APIDeclarationSequenceSuffix
	case 35:
		return APIDeclarationSequenceInRegex
	case 36:
		return APIDeclarationSequenceToRegex
	case 37:
		return APIDeclarationSequenceAt
	case 38:
		return APIDeclarationSequenceExtract
	case 39:
		return APIDeclarationSequenceIndex
	case 40:
		return APIDeclarationRegexEmptySet
	case 41:
		return APIDeclarationRegexFullSet
	default:
		return APIDeclarationInternal
	}
}
