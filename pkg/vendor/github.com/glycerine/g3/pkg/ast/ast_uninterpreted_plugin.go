package ast

// Source: src/ast/ast.h:1026; src/ast/bv_decl_plugin.h:271-285;
// src/ast/arith_decl_plugin.h:202-217.

// IsConsideredUninterpreted dispatches the declaration plugin hook. Ordinary
// null-family functions are handled separately by consumers of this hook.
func (m *ASTManager) IsConsideredUninterpreted(d ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(d, ASTFuncDecl)
	if e != nil {
		return false, e
	}
	fid, kind, e := m.declIdentity(r)
	if e != nil {
		return false, e
	}
	if fid < 0 || !m.pluginAt(fid).present {
		return false, nil
	}
	switch m.pluginAt(fid).kind {
	case astBitVectorPlugin:
		switch kind {
		case 12, 13, 14, 15, 16:
			return true, nil
		}
	case astArithmeticPlugin:
		switch kind {
		case 33, 34, 35:
			return true, nil
		}
	}
	return false, nil
}

// Source: src/ast/ast.h:1312; src/ast/bv_decl_plugin.h:298;
// src/ast/arith_decl_plugin.h:235. Inspect declaration identity without
// constructing an application or reading its numerical parameters.
func (m *ASTManager) IsNumeralDeclaration(d ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(d, ASTFuncDecl)
	if e != nil {
		return false, e
	}
	fid, kind, e := m.declIdentity(r)
	if e != nil {
		return false, e
	}
	if kind != 0 || fid < 0 || !m.pluginAt(fid).present {
		return false, nil
	}
	p := m.pluginAt(fid).kind
	return p == astBitVectorPlugin || p == astArithmeticPlugin, nil
}
