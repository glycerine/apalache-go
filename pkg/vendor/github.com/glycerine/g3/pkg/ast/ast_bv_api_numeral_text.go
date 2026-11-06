package ast

// Source: src/api/api_numeral.cpp:158-165,187-194;
// src/ast/bv_decl_plugin.cpp:778-790.

import "errors"

// BitVectorNumeralText reads the unsigned rational value returned by the
// source API's BitVec numeral probe. Width belongs to the declaration; the
// displayed rational is detached from both the temporary and the AST node.
func (m *ASTManager) BitVectorNumeralText(h ASTHandle) (value string, matched bool, result error) {
	s := m.NumeralService()
	if s == nil {
		return "", false, ErrASTHandle
	}
	r, err := s.NewValue()
	if err != nil {
		return "", false, err
	}
	defer func() {
		result = errors.Join(result, s.Del(&r))
		if result != nil {
			value = ""
		}
	}()
	var width uint32
	matched, result = m.BitVectorNumeralValue(h, &r, &width)
	if result != nil || !matched {
		return "", matched, result
	}
	value, result = s.String(&r)
	return value, true, result
}
