package polynomial

// Source: src/math/polynomial/polynomial.cpp:4905-5091; src/util/obj_ref.h.

type polynomialPseudoDivisionInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialPseudoDivisionScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesPseudoInvocation(r *polynomialPseudoDivisionInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.pseudoDivision && r.invocation != 0 && r.invocation == r.frame.invocation && r.frame.active
}
func (m *MultivariatePolynomialCore) cleanupPseudoInvocation(r *polynomialPseudoDivisionInvocationReceipt, abort bool) error {
	if !m.matchesPseudoInvocation(r) {
		return nil
	}
	return m.pseudoCleanupPhase(true, abort)
}
func (m *MultivariatePolynomialCore) pseudoInvocationSourceCause(r *polynomialPseudoDivisionInvocationReceipt) error {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame != &m.pseudoDivision || r.invocation == 0 || r.invocation != r.frame.invocation {
		return nil
	}
	return r.frame.sourceCause
}
