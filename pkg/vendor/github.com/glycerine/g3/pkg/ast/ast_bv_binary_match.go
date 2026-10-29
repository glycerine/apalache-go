package ast

// Source: src/ast/bv_decl_plugin.h:353-368; src/ast/ast.h:530-534.

// Operand results borrow existing nodes. Family/kind and arity guards precede
// every output read; matched outputs publish argument zero before argument one.
func (m *ASTManager) bitVectorBinaryOperands(h ASTHandle, kind int32, left, right *ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	found, e := m.bitVectorAppKind(r, kind)
	if e != nil || !found {
		return false, e
	}
	app := m.appAt(r.slot)
	if app.count != 2 {
		return false, nil
	}
	a, e := m.readInternal(app.args, 0)
	if e != nil {
		return false, e
	}
	if left == nil {
		return false, ErrASTDomain
	}
	*left = m.external(a)
	b, e := m.readInternal(app.args, 1)
	if e != nil {
		return false, e
	}
	if right == nil {
		return false, ErrASTDomain
	}
	*right = m.external(b)
	return true, nil
}

func (m *ASTManager) BitVectorAddOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 4, left, right)
}

func (m *ASTManager) BitVectorMulOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 6, left, right)
}

func (m *ASTManager) BitVectorSLEOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 23, left, right)
}

func (m *ASTManager) BitVectorULEOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 22, left, right)
}

func (m *ASTManager) BitVectorSHLOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 45, left, right)
}

func (m *ASTManager) BitVectorURemOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 10, left, right)
}

func (m *ASTManager) BitVectorSRemOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 9, left, right)
}

func (m *ASTManager) BitVectorSDivOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 7, left, right)
}

func (m *ASTManager) BitVectorUDivOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 8, left, right)
}

func (m *ASTManager) BitVectorSModOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 11, left, right)
}

func (m *ASTManager) BitVectorURemIOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 20, left, right)
}

func (m *ASTManager) BitVectorSRemIOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 19, left, right)
}

func (m *ASTManager) BitVectorSDivIOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 17, left, right)
}

func (m *ASTManager) BitVectorUDivIOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 18, left, right)
}

func (m *ASTManager) BitVectorSModIOperands(h ASTHandle, left, right *ASTHandle) (bool, error) {
	return m.bitVectorBinaryOperands(h, 21, left, right)
}
