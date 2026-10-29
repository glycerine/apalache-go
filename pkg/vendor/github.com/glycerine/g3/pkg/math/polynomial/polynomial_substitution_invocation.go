package polynomial

// Source: polynomial.cpp, concrete integer substitution reached by mod_gcd_rec.

type polynomialSubstitutionInvocationReceipt struct {
	core       *MultivariatePolynomialCore
	frame      *polynomialSubstitutionScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesSubstitutionInvocation(r *polynomialSubstitutionInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.core == m && r.frame == &m.substitution.scope && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (m *MultivariatePolynomialCore) substitutionInvocationPending(r *polynomialSubstitutionInvocationReceipt) bool {
	if !m.matchesSubstitutionInvocation(r) {
		return false
	}
	d := &m.substitution
	return d.scope.active || m.substitutionNumericPending() || m.pending.substitutionOwned || m.powerLocal.substitutionOwned || d.borrowedReceipt != 0 || d.generatedCalleeReceipt != 0 || d.generatedReceipt != (Monomial{})
}
func (m *MultivariatePolynomialCore) substitutionInvocationSourceCause(r *polynomialSubstitutionInvocationReceipt) error {
	if !m.matchesSubstitutionInvocation(r) {
		return nil
	}
	return r.frame.sourceCause
}
func (m *MultivariatePolynomialCore) cleanupSubstitutionInvocation(r *polynomialSubstitutionInvocationReceipt, abort bool) error {
	if !m.matchesSubstitutionInvocation(r) {
		return nil
	}
	if r.frame.executing {
		return ErrPolynomialPending
	}
	if !abort {
		return m.substitutionCleanupPhase(true, false)
	}
	return m.CleanupSubstitution()
}

func (m *MultivariatePolynomialCore) substitutionNumericPending() bool {
	s := &m.substitution.scope
	if s.modularPowerPending || s.powerNumeric.roots.PendingObject().OwnerID() != 0 {
		return true
	}
	for _, b := range s.powerNumeric.roots.PendingAllocatorBlocks() {
		if b.OwnerID() != 0 {
			return true
		}
	}
	return false
}
