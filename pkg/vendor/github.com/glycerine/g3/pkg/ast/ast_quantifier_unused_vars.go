package ast

// Source: src/ast/ast.h quantifier::may_have_unused_vars/set_no_unused_vars.

func (m *ASTManager) QuantifierMayHaveUnusedVars(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return false, e
	}
	return m.quantifierAt(r.slot).unusedVars, nil
}

func (m *ASTManager) SetQuantifierNoUnusedVars(h ASTHandle) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return e
	}
	m.quantifierAt(r.slot).unusedVars = false
	return nil
}
