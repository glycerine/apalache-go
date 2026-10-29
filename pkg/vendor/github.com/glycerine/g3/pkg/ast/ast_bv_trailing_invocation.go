package ast

// Source: src/ast/rewriter/bv_trailing.cpp:213-234,311-327.

// ASTBitVectorNumeralInvocation names the actual numerical child of a
// structural caller. A failed call cannot be reused, even after owning cleanup.
type ASTBitVectorNumeralInvocation struct {
	self            *ASTBitVectorNumeralInvocation
	manager         *ASTManager
	receipt         astBVNormalizationReceipt
	active, stopped bool
}

func (c *ASTBitVectorNumeralInvocation) prepare(m *ASTManager) error {
	if c == nil || c.self != nil && (c.self != c || c.manager != m) {
		return ErrASTHandle
	}
	if c.active || c.stopped || c.receipt.frame != nil && !c.receipt.retired {
		return ErrASTPending
	}
	*c = ASTBitVectorNumeralInvocation{self: c, manager: m, active: true}
	return nil
}
func (c *ASTBitVectorNumeralInvocation) finish(body *error) {
	c.active = false
	c.stopped = *body != nil
}
func (c *ASTBitVectorNumeralInvocation) valid(m *ASTManager) error {
	if c == nil || c.self != c || c.manager != m {
		return ErrASTHandle
	}
	if c.active {
		return ErrASTPending
	}
	return nil
}

func (m *ASTManager) CountBitVectorNumeralTrailingZerosWithInvocation(arg ASTHandle, c *ASTBitVectorNumeralInvocation) (bool, uint32, error) {
	if c == nil {
		return false, 0, ErrASTHandle
	}
	return m.countBitVectorNumeralTrailingZeros(arg, c)
}
func (m *ASTManager) RemoveBitVectorNumeralTrailingZerosWithInvocation(arg ASTHandle, n uint32, out ASTRef, c *ASTBitVectorNumeralInvocation) (bool, uint32, error) {
	if c == nil {
		return false, 0, ErrASTHandle
	}
	return m.removeBitVectorNumeralTrailingZeros(arg, n, out, c)
}
func (m *ASTManager) BitVectorNumeralInvocationSourceReady(c *ASTBitVectorNumeralInvocation) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if c != nil && c.self == nil {
		return nil
	}
	if e := c.valid(m); e != nil {
		return e
	}
	return c.receipt.sourceReady(m.bitVectorPlugin)
}
func (m *ASTManager) CleanupBitVectorNumeralInvocation(c *ASTBitVectorNumeralInvocation) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if c != nil && c.self == nil {
		return nil
	}
	if e := c.valid(m); e != nil {
		return e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	if e := c.receipt.cleanup(m.bitVectorPlugin, false); e != nil {
		return e
	}
	return c.receipt.cleanup(m.bitVectorPlugin, true)
}
