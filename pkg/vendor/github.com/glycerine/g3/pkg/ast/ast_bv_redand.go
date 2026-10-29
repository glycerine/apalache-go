package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2048-2056;
// src/ast/rewriter/bv_rewriter.h:177; src/util/rational.h:462-464.

// ReduceBitVectorAND keeps the numerical condition's source temporaries and
// function-local rational alive through result publication. It returns false
// without accessing out for nonnumeral expressions.
func (m *ASTManager) ReduceBitVectorAND(arg ASTHandle, out ASTRef) (matched bool, result error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(arg)
	if e != nil {
		return false, e
	}
	found, e := m.bitVectorAppKind(r, 0)
	if e != nil || !found {
		return false, e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return true, e
	}
	f.allOne = true
	defer f.finishNumeral(&result, nil)
	all, e := m.bitVectorAllOneCondition(f, r)
	if e != nil {
		return true, e
	}
	value := uint32(0)
	if all {
		value = 1
	}
	node, e := m.bitVectorNumeralUnsignedCaptured(value, 1, &f.normalizationChild)
	if e != nil {
		return true, e
	}
	// The entire assignment is one full expression: its power/one/subtraction
	// temporaries end after assignRef; the local numeral ends at function exit.
	return true, m.assignRef(out, node)
}
