package ast

// Source: src/ast/ast.h ast_manager::is_rec_fun_def/rec_fun_qid and
// src/ast/ast.cpp reserved recursive-definition identifier.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

func (m *ASTManager) RecursiveFunctionQID() (usymbol.Symbol, error) {
	if _, err := m.ProofsEnabled(); err != nil {
		return usymbol.Symbol{}, err
	}
	return m.Symbols.Intern(":rec-fun")
}

func (m *ASTManager) IsRecursiveFunctionDefinition(q ASTHandle) (bool, error) {
	qid, _, err := m.GetQuantifierIDs(q)
	if err != nil || qid.IsNull() || qid.IsNumerical() {
		return false, err
	}
	name, present, err := m.Symbols.BareString(qid)
	return present && name == ":rec-fun", err
}
