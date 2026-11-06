package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind sequence-family switch;
// src/ast/seq_decl_plugin.h seq_op_kind.

import "github.com/glycerine/g3/pkg/ast"

const (
	APIDeclarationSequenceUnit APIDeclarationKind = 400 + iota
	APIDeclarationSequenceEmpty
	APIDeclarationSequenceConcat
	APIDeclarationSequencePrefix
	APIDeclarationSequenceSuffix
	APIDeclarationSequenceContains
	APIDeclarationSequenceExtract
	APIDeclarationSequenceReplace
	APIDeclarationSequenceAt
	APIDeclarationSequenceLength
	APIDeclarationSequenceIndex
	APIDeclarationSequenceToRegex
	APIDeclarationSequenceInRegex
	APIDeclarationStringToInteger
	APIDeclarationIntegerToString
	APIDeclarationRegexPlus
	APIDeclarationRegexStar
	APIDeclarationRegexOption
	APIDeclarationRegexConcat
	APIDeclarationRegexUnion
	APIDeclarationRegexIntersect
	APIDeclarationRegexLoop
	APIDeclarationRegexFullSet
	APIDeclarationRegexEmptySet
)

// SequenceDeclarationKind classifies the public sequence, string, and regular
// expression operations retained by Z3_get_decl_kind.
func (c *SMTContext) SequenceDeclarationKind(value ast.ASTHandle) (APIDeclarationKind, error) {
	manager, family, operation, err := c.apiDeclarationFamilyOperation(value)
	if err != nil {
		return APIDeclarationInternal, err
	}
	sequence, err := manager.SequenceFamilyID()
	if err != nil {
		return APIDeclarationInternal, err
	}
	if family != sequence {
		return APIDeclarationInternal, ast.ErrASTDomain
	}
	return apiSequenceDeclarationKind(operation), nil
}

func apiSequenceDeclarationKind(operation int32) APIDeclarationKind {
	if kind := apiSequenceCoreDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiSequenceRelationDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiSequenceIndexedDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiSequenceStringDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	return apiRegexDeclarationKind(operation)
}
