package polynomial

// Source: src/math/polynomial/polynomial.cpp:4600-4614; src/util/obj_ref.h.

type polynomialPowerInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialPowerScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesPowerInvocation(r *polynomialPowerInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.powerLocal && r.invocation != 0 && r.invocation == r.frame.generation && r.frame.active
}
func (m *MultivariatePolynomialCore) cleanupPowerInvocation(r *polynomialPowerInvocationReceipt, abort bool) error {
	if !m.matchesPowerInvocation(r) {
		return nil
	}
	return m.cleanupPolynomialPowerPhase(true, abort)
}
func (m *MultivariatePolynomialCore) powerInvocationSourceCause(r *polynomialPowerInvocationReceipt) error {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame != &m.powerLocal || r.invocation == 0 || r.invocation != r.frame.generation {
		return nil
	}
	return r.frame.sourceCause
}

func (m *MultivariatePolynomialCore) powerInvocationBodyCause(r *polynomialPowerInvocationReceipt) error {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame != &m.powerLocal || r.invocation == 0 || r.invocation != r.frame.generation {
		return nil
	}
	return r.frame.bodyCause
}
