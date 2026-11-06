package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1960-1963;
// src/util/rational.h:43,306-308.

// BitVectorNumeralOneWithInvocation passes borrowed rational::one() to the
// original numeral factory. Its parameter copy is the only caller value;
// there is no outer rational(1) constructor or temporary destructor.
func (m *ASTManager) BitVectorNumeralOneWithInvocation(bits uint32, c *ASTBitVectorNumeralInvocation) (node ASTHandle, result error) {
	if c == nil {
		return node, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return node, e
	}
	defer m.mu.Unlock()
	if e := c.prepare(m); e != nil {
		return node, e
	}
	defer c.finish(&result)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	r, e := m.bitVectorNumeralOneCaptured(bits, &c.receipt)
	return m.external(r), e
}

func (m *ASTManager) bitVectorNumeralOneCaptured(bits uint32, receipt *astBVNormalizationReceipt) (node astNodeRef, result error) {
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return node, e
	}
	*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	defer f.finishNumeral(&result, receipt)
	f.values[0], e = m.numerals.NewValue()
	if e != nil {
		return node, e
	}
	f.placed[0] = true
	if e = m.numerals.CopyOneWithInvocation(&f.values[0], &f.children[2]); e != nil {
		return node, e
	}
	f.complete[0] = true
	return m.bitVectorNumeralAppCore(f, bits)
}
