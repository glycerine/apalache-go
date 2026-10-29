package ast

// Source: src/ast/ast_smt2_pp.cpp:370-383,628-630.

import "strings"

// SMT2PPStringLiteral formats a sequence string constant. matched is false
// for expressions outside the string-constant specialization.
func (m *ASTManager) SMT2PPStringLiteral(h ASTHandle) (format ASTHandle, matched bool, err error) {
	value, matched, err := m.SequenceStringValue(h)
	if err != nil || !matched {
		return ASTHandle{}, matched, err
	}
	encoded := encodeSequenceString(value)
	encoded = strings.ReplaceAll(encoded, `"`, `""`)
	format, err = m.FormatString(`"` + encoded + `"`)
	return format, true, err
}
