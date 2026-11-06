package polynomial

// CleanupBerlekampMatrix reclaims the exact matrix retained by interrupted
// source destruction, without retiring caller polynomials or the manager.
func (m *UPolynomialManager) CleanupBerlekampMatrix() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if m.factorTrace != nil && m.factorTrace.executing || m.displayExecuting() || m.config.Integer.RuntimeOwner().FailureTransportActive() {
		return ErrUPolynomialCleanupPending
	}
	return m.cleanupBerlekamp()
}

// cleanupBerlekamp is the owning abort of the exact matrix retained by a
// stopped lexical destructor. It cannot adopt a different display invocation.
func (m *UPolynomialManager) cleanupBerlekamp() error {
	q := m.pendingBerlekamp
	if q == nil {
		return nil
	}
	if q.m != m {
		return ErrUPolynomialOwner
	}
	if q.displaying || q.destroying || q.numeric.executing || q.numeric.cleaning {
		return ErrUPolynomialCleanupPending
	}
	if e := q.outcomeFactors.cleanup(); e != nil {
		return e
	}
	if e := m.cleanupDisplayReceipt(&q.entryTrace); e != nil {
		return e
	}
	q.destroying = true
	defer func() { q.destroying = false }()
	if e := q.numeric.cleanup(false); e != nil {
		return e
	}
	if e := q.closeOwnedMembers(); e != nil {
		return e
	}
	if e := q.numeric.cleanup(true); e != nil {
		return e
	}
	if !q.numeric.pending() && m.pendingBerlekamp == q {
		m.pendingBerlekamp = nil
	}
	return nil
}
