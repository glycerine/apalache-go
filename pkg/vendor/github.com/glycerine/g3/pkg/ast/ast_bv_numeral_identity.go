package ast

// Source: src/ast/bv_decl_plugin.h:298.

// IsBitVectorNumeral is the parameter-free app/family/kind overload. It does
// not read, copy or validate the declaration's numerical parameters.
func (m *ASTManager) IsBitVectorNumeral(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.isBitVectorNumeral(r)
}
