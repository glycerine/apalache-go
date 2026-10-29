package ast

// Source: src/ast/bv_decl_plugin.h:307-310.

// Each getter reads only its original declaration parameter. Callers can keep
// the source short-circuit order without recognizing/reading all extract fields.
func (m *ASTManager) BitVectorExtractHigh(h ASTHandle) (uint32, error) {
	return m.bitVectorExtractBound(h, 0)
}
func (m *ASTManager) BitVectorExtractLow(h ASTHandle) (uint32, error) {
	return m.bitVectorExtractBound(h, 1)
}
func (m *ASTManager) bitVectorExtractBound(h ASTHandle, index uint32) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return 0, e
	}
	if ASTSourceDebug {
		yes, e := m.bitVectorAppKind(r, 40)
		if e != nil {
			return 0, e
		}
		if !yes {
			return 0, ErrASTDomain
		}
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return 0, ErrASTDomain
	}
	v, e := m.bitVectorIntegerParameter(r, index)
	return uint32(v), e
}
