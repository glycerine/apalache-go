package ast

// Source: src/ast/bv_decl_plugin.h:397-434.

// mkBitVectorUtilityApp uses the registered BV family's original declaration
// dispatch and application factory. Its argument slice is borrowed.
func (m *ASTManager) mkBitVectorUtilityApp(kind int32, args []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.bitVectorPlugin == nil {
		return ASTHandle{}, ErrASTDomain
	}
	r, e := m.mkFamilyApp(m.bitVectorPlugin.family, kind, nil, args)
	return m.external(r), e
}

func (m *ASTManager) MkBitVectorULE(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(22, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorSLE(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(23, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorURem(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(10, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorUDiv(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(8, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorSDiv(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(7, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorSMod(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(11, []ASTHandle{left, right})
}

// MkBitVectorUDivInternal is the source OP_BUDIV_I form used when the caller
// has already proved that the divisor is nonzero.
func (m *ASTManager) MkBitVectorUDivInternal(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(18, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorSDivInternal(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(17, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorSRemInternal(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(19, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorSModInternal(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(21, []ASTHandle{left, right})
}

// MkBitVectorSRemZero is the source's uninterpreted value for signed
// remainder by zero.
func (m *ASTManager) MkBitVectorSRemZero(arg ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(14, []ASTHandle{arg})
}

func (m *ASTManager) MkBitVectorSRem(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(9, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorAdd(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(4, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorSub(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(5, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorMul(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(6, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorSHL(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(45, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorASHR(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(47, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorLSHR(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(46, []ASTHandle{left, right})
}

func (m *ASTManager) MkBitVectorExtendedRotateLeft(value, amount ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(50, []ASTHandle{value, amount})
}

func (m *ASTManager) MkBitVectorExtendedRotateRight(value, amount ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(51, []ASTHandle{value, amount})
}

// The source overflow utility intentionally passes its operands in reverse order.
func (m *ASTManager) MkBitVectorSignedMulNoOverflow(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(53, []ASTHandle{right, left})
}

// The source overflow utility intentionally passes its operands in reverse order.
func (m *ASTManager) MkBitVectorSignedMulNoUnderflow(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(54, []ASTHandle{right, left})
}

// The source overflow utility intentionally passes its operands in reverse order.
func (m *ASTManager) MkBitVectorUnsignedMulNoOverflow(left, right ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(52, []ASTHandle{right, left})
}

func (m *ASTManager) MkBitVectorNot(arg ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(32, []ASTHandle{arg})
}

func (m *ASTManager) MkBitVectorReductionOr(arg ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(42, []ASTHandle{arg})
}

func (m *ASTManager) MkBitVectorReductionAnd(arg ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(43, []ASTHandle{arg})
}

func (m *ASTManager) MkBitVectorNeg(arg ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(3, []ASTHandle{arg})
}

func (m *ASTManager) MkBitVectorConcat(args []ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(37, args)
}

func (m *ASTManager) MkBitVectorOr(args []ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(31, args)
}

func (m *ASTManager) MkBitVectorAnd(args []ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(30, args)
}

func (m *ASTManager) MkBitVectorXor(args []ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(33, args)
}

func (m *ASTManager) MkBitVectorFromBools(args []ASTHandle) (ASTHandle, error) {
	return m.mkBitVectorUtilityApp(56, args)
}

func (m *ASTManager) MkBitVectorConcatPair(left, right ASTHandle) (ASTHandle, error) {
	return m.MkBitVectorConcat([]ASTHandle{left, right})
}
