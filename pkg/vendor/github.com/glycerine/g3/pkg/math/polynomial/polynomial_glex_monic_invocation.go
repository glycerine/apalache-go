package polynomial

// The source caller retains only an accepted, concrete monic invocation.
type polynomialMonicInvocationReceipt struct {
	manager    *MultivariatePolynomialManager
	frame      *polynomialMonicScope
	invocation uint64
}

func (m *MultivariatePolynomialManager) matchesMonicInvocation(r *polynomialMonicInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.monic && r.invocation != 0 && r.invocation == r.frame.invocation && r.frame.active
}
func (m *MultivariatePolynomialManager) cleanupMonicInvocation(r *polynomialMonicInvocationReceipt, abort bool) error {
	if !m.matchesMonicInvocation(r) {
		return nil
	}
	return m.cleanupGlexMonicPhase(abort)
}
