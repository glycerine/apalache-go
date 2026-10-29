package ast

// Source: src/ast/ast_smt2_pp.cpp:370-389,637-639.

import "strconv"

// SMT2PPDatalogLiteral formats a finite-domain numeral. matched is false for
// expressions outside the Datalog numeral specialization.
func (m *ASTManager) SMT2PPDatalogLiteral(h ASTHandle) (format ASTHandle, matched bool, err error) {
	value, matched, err := m.DatalogNumeralValue(h)
	if err != nil || !matched {
		return ASTHandle{}, matched, err
	}
	format, err = m.FormatString(strconv.FormatUint(value, 10))
	return format, true, err
}
