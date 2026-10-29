package polynomial

// Source: src/math/polynomial/polynomial.cpp:5154-5194;
// src/util/scoped_numeral.h; src/util/obj_ref.h.

// Names only the actual embedded source scope and its guarded invocation.
type polynomialDividesInvocationReceipt struct {
	trace      polynomialDividesTraceInvocation
	prelude    polynomialArithmeticInvocationReceipt
	core       *MultivariatePolynomialCore
	frame      *polynomialDividesScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesDividesInvocationHistory(r *polynomialDividesInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.core == m && r.frame == &m.dividesLocal && r.invocation != 0 && r.frame.invocation == r.invocation
}
func (m *MultivariatePolynomialCore) matchesDividesInvocation(r *polynomialDividesInvocationReceipt) bool {
	return m.matchesDividesInvocationHistory(r) && r.frame.active
}

func (m *MultivariatePolynomialCore) dividesInvocationSourceCause(r *polynomialDividesInvocationReceipt) error {
	if r != nil {
		if e := m.dividesTraceSourceCause(&r.trace); e != nil {
			return e
		}
	}
	if !m.matchesDividesInvocationHistory(r) {
		return nil
	}
	return r.frame.sourceCause
}
func (m *MultivariatePolynomialCore) cleanupDividesInvocation(r *polynomialDividesInvocationReceipt, abort bool) error {
	if r != nil {
		if e := m.cleanupDividesTrace(&r.trace); e != nil {
			return e
		}
	}
	if !m.matchesDividesInvocationHistory(r) {
		return nil
	}
	s := r.frame
	if s.executing {
		return ErrPolynomialPending
	}
	if e := m.cleanupArithmeticInvocation(&r.prelude, false); e != nil {
		return e
	}
	if e := m.cleanupDividesPhase(s, abort); e != nil {
		s.cleanupCause = e
		return e
	}
	if abort {
		if e := m.cleanupArithmeticInvocation(&r.prelude, true); e != nil {
			s.cleanupCause = e
			return e
		}
		s.active = false
	}
	return nil
}

func (m *MultivariatePolynomialCore) dividesInvocationPending(r *polynomialDividesInvocationReceipt) bool {
	if r != nil && m.dividesTraceInvocationPending(&r.trace) {
		return true
	}
	if !m.matchesDividesInvocationHistory(r) {
		return false
	}
	return r.frame.active || m.matchesArithmeticInvocation(&r.prelude)
}
