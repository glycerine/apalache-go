package polynomial

// Source: src/math/polynomial/polynomial.cpp:5954-6068;
// src/util/obj_ref.h; src/util/scoped_numeral.h.

type polynomialSqrtInvocationReceipt struct {
	manager     *MultivariatePolynomialManager
	frame       *polynomialSquareRootScope
	invocation  uint64
	orphanDrain uint32
}

func (m *MultivariatePolynomialManager) matchesSqrtInvocationHistory(r *polynomialSqrtInvocationReceipt) bool {
	return m != nil && m.self == m && m.core != nil && m.core.self == m.core && r != nil && r.manager == m && r.frame == &m.squareRoot && r.invocation != 0 && r.invocation == m.squareRoot.invocation
}
func (m *MultivariatePolynomialManager) matchesSqrtInvocation(r *polynomialSqrtInvocationReceipt) bool {
	return m.matchesSqrtInvocationHistory(r) && r.frame.active
}
func (m *MultivariatePolynomialManager) sqrtInvocationSourceCause(r *polynomialSqrtInvocationReceipt) error {
	if m.matchesSqrtInvocationHistory(r) {
		return r.frame.sourceCause
	}
	return nil
}
func (m *MultivariatePolynomialManager) cleanupSqrtOrphanInvocation(invocation uint64, cursor *uint32) error {
	s := &m.squareRoot
	for *cursor < uint32(len(s.orphans)) {
		i := *cursor
		if s.orphans[i] != (Monomial{}) && (invocation == 0 || s.orphanInvocations[i] == invocation) {
			if e := m.core.monomials.CleanupValue(&s.orphans[i]); e != nil {
				s.cleanupCause = e
				return e
			}
			s.orphanInvocations[i] = 0
		}
		*cursor++
	}
	return nil
}
func (m *MultivariatePolynomialManager) cleanupSqrtInvocation(r *polynomialSqrtInvocationReceipt, abort bool) error {
	if r == nil || r.manager != m || r.frame != &m.squareRoot || r.invocation == 0 {
		return nil
	}
	if e := m.identity(); e != nil {
		return e
	}
	if m.squareRoot.executing {
		return ErrPolynomialPending
	}
	if r.invocation == m.squareRoot.invocation {
		return m.cleanupSqrtModeOwned(abort, r.invocation, &r.orphanDrain)
	}
	if !abort {
		return nil
	}
	return m.cleanupSqrtOrphanInvocation(r.invocation, &r.orphanDrain)
}
