package ast

// Source: src/ast/bv_decl_plugin.h:304,313,315; src/ast/ast.h:1308.

// IsBitVectorOperation reads only application/family/kind identity.
func (m *ASTManager) IsBitVectorOperation(h ASTHandle, kind int32) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.bitVectorAppKind(r, kind)
}
