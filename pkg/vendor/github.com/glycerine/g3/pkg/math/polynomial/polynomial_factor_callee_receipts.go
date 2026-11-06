package polynomial

// Source: src/math/polynomial/polynomial.cpp:4507-4528,5114-5156;
// src/util/scoped_numeral.h; src/util/obj_ref.h.

// Cold owning continuation identities are acquired only at the actual guarded
// invocation. The source NUM arithmetic and persistent buffers stay unchanged.
type polynomialDerivativeInvocationReceipt struct {
	orphanDrain uint32
	manager     *MultivariatePolynomialManager
	frame       *polynomialDerivativeScope
	invocation  uint64
}
type polynomialExactDivInvocationReceipt struct {
	trace      polynomialExactDivTraceInvocation
	prelude    polynomialArithmeticInvocationReceipt
	core       *MultivariatePolynomialCore
	frame      *polynomialExactDivisionScope
	invocation uint64
}

func (m *MultivariatePolynomialManager) matchesDerivativeInvocationHistory(r *polynomialDerivativeInvocationReceipt) bool {
	return m != nil && m.self == m && m.core != nil && m.core.self == m.core && r != nil && r.manager == m && r.frame == &m.derivative && r.invocation != 0 && r.frame.invocation == r.invocation
}
func (m *MultivariatePolynomialManager) matchesDerivativeInvocation(r *polynomialDerivativeInvocationReceipt) bool {
	return m.matchesDerivativeInvocationHistory(r) && r.frame.active
}
func (m *MultivariatePolynomialManager) derivativeInvocationSourceCause(r *polynomialDerivativeInvocationReceipt) error {
	if m.matchesDerivativeInvocationHistory(r) {
		return r.frame.sourceCause
	}
	return nil
}
func (m *MultivariatePolynomialManager) cleanupDerivativeInvocation(r *polynomialDerivativeInvocationReceipt, abort bool) error {
	if r == nil || r.manager != m || r.frame != &m.derivative || r.invocation == 0 {
		return nil
	}
	if m.matchesDerivativeInvocation(r) {
		return m.cleanupDerivativeModeOwned(abort, r.invocation, &r.orphanDrain)
	}
	if !abort {
		return nil
	}
	if e := m.identity(); e != nil {
		return e
	}
	if m.derivative.executing {
		return ErrPolynomialPending
	}
	if e := m.cleanupDerivativeOrphanInvocation(r.invocation, &r.orphanDrain); e != nil {
		return e
	}
	if m.derivative.invocation == r.invocation {
		m.releaseDerivativeRetirementIfEmpty()
	}
	return nil
}
func (m *MultivariatePolynomialCore) matchesExactDivInvocation(r *polynomialExactDivInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.core == m && r.frame == &m.exactDivision && r.invocation != 0 && r.frame.invocation == r.invocation && r.frame.kind == 1 && r.frame.active
}
func (m *MultivariatePolynomialCore) exactDivInvocationSourceCause(r *polynomialExactDivInvocationReceipt) error {
	if r == nil {
		return nil
	}
	if e := m.exactDivTraceSourceCause(&r.trace); e != nil {
		return e
	}
	if e := m.arithmeticInvocationSourceCause(&r.prelude); e != nil {
		return e
	}
	if m != nil && m.self == m && r.core == m && r.frame == &m.exactDivision && r.invocation != 0 && r.frame.invocation == r.invocation && r.frame.kind == 1 {
		return r.frame.sourceCause
	}
	return nil
}
func (m *MultivariatePolynomialCore) cleanupExactDivInvocation(r *polynomialExactDivInvocationReceipt, abort bool) error {
	if r == nil {
		return nil
	}
	if e := m.cleanupExactDivTrace(&r.trace); e != nil {
		return e
	}
	if e := m.cleanupArithmeticInvocation(&r.prelude, false); e != nil {
		return e
	}
	if m.matchesExactDivInvocation(r) {
		if e := m.exactDivisionCleanupPhase(true, abort); e != nil {
			return e
		}
	}
	if abort {
		return m.cleanupArithmeticInvocation(&r.prelude, true)
	}
	return nil
}

func (m *MultivariatePolynomialCore) exactDivInvocationPending(r *polynomialExactDivInvocationReceipt) bool {
	return r != nil && (m.exactDivTraceInvocationPending(&r.trace) || m.matchesExactDivInvocation(r) || m.matchesArithmeticInvocation(&r.prelude))
}
