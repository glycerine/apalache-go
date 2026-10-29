package ast

// Source: src/ast/arith_decl_plugin.cpp:511-559.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

func (m *ASTManager) GetArithmeticSortNames(names *ASTBuiltinNames, logic usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.arithmeticPlugin == nil {
		return ErrASTHandle
	}
	converted := false
	for _, text := range []string{"NRA", "QF_NRA", "QF_UFNRA"} {
		yes, e := m.Symbols.EqualString(logic, &text)
		if e != nil {
			return e
		}
		if yes {
			converted = true
			break
		}
	}
	if converted {
		m.arithmeticPlugin.convertIntNumerals = true
	} else {
		if e := m.appendBuiltinName(names, "Int", 1); e != nil {
			return e
		}
	}
	return m.appendBuiltinName(names, "Real", 0)
}

func (m *ASTManager) GetArithmeticOpNames(names *ASTBuiltinNames, logic usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.arithmeticPlugin == nil {
		return ErrASTHandle
	}
	texts := [...]string{"<=", ">=", "<", ">", "+", "-", "~", "*", "/", "div", "rem", "mod", "to_real", "to_int", "is_int", "abs", "^", "sin", "cos", "tan", "asin", "acos", "atan", "sinh", "cosh", "tanh", "asinh", "acosh", "atanh", "pi", "euler"}
	for i := 0; i < 16; i++ {
		if e := m.appendBuiltinName(names, texts[i], int32(i+2)); e != nil {
			return e
		}
	}
	all, e := m.builtinAllLogic(logic)
	if e != nil || !all {
		return e
	}
	for i := 16; i < len(texts); i++ {
		if e := m.appendBuiltinName(names, texts[i], int32(i+2)); e != nil {
			return e
		}
	}
	return nil
}
