package ast

// Source: src/ast/datatype_decl_plugin.cpp:458-466.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

func (m *ASTManager) GetDatatypeOpNames(names *ASTBuiltinNames, logic usymbol.Symbol) error {
	if err := m.lock(); err != nil {
		return err
	}
	defer m.mu.Unlock()
	if names == nil || m.datatypePlugin == nil {
		return ErrASTHandle
	}
	if err := m.appendBuiltinName(names, "is", ASTDatatypeIs); err != nil {
		return err
	}
	all, err := m.builtinAllLogic(logic)
	if err != nil || !all {
		return err
	}
	return m.appendBuiltinName(names, "update-field", ASTDatatypeUpdateField)
}
