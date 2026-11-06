package ast

// Source: src/ast/ast.h:934-938; src/util/vector.h:59-100,349-363.

import (
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type ASTBuiltinName struct {
	Kind int32
	Name usymbol.Symbol
}

// Caller-owned collection of borrowed names. Items borrows the backing;
// symbols borrow the manager's symbol service.
type ASTBuiltinNames struct {
	items []ASTBuiltinName
}

func (v *ASTBuiltinNames) Items() []ASTBuiltinName { return v.items }
func (v *ASTBuiltinNames) Reset()                  { v.items = v.items[:0] }
func (v *ASTBuiltinNames) Close()                  { v.items = nil }

func (m *ASTManager) appendBuiltinName(names *ASTBuiltinNames, text string, kind int32) error {
	s, e := m.Symbols.Intern(text)
	if e != nil {
		return e
	}
	names.items = append(names.items, ASTBuiltinName{Kind: kind, Name: s})
	return nil
}

// Source: src/ast/arith_decl_plugin.cpp:542;
// src/ast/bv_decl_plugin.cpp:673,720.
func (m *ASTManager) builtinAllLogic(logic usymbol.Symbol) (bool, error) {
	if logic.IsNull() {
		return true, nil
	}
	all, e := m.Symbols.Intern("ALL")
	return logic == all, e
}
