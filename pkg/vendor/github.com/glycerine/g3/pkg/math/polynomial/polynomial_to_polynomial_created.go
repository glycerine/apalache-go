package polynomial

// Source: src/math/polynomial/polynomial.cpp:2582-2594,862-884.

func (m *MultivariatePolynomialCore) generatedVariableToPolynomial(v, k uint32, r *polynomialToPolynomialInvocationReceipt) (Monomial, error) {
	if r == nil || !r.manager.matchesToPolynomialInvocation(r) || r.manager.core != m || !r.frame.executing || m.callbackDepth != 0 {
		return Monomial{}, ErrPolynomialOwner
	}
	var outcome monomialFactoryOutcome
	a, e := m.generatedVariableWithOutcome(v, k, &outcome)
	s := r.frame
	if outcome.allocatorInvocation.invocation != 0 {
		s.monomialAllocator = outcome.allocatorInvocation
	}
	if outcome.started && e != nil {
		s.calleeGeneration = outcome.generation
		m.monomials.creation.toPolynomialParent = s
		m.monomials.creation.toPolynomialInvocation = r.invocation
	}
	if outcome.created && e == nil {
		s.generated = a
		l, x := m.monomials.location(a)
		if x != nil {
			return a, x
		}
		l.toPolynomialCohort = s
		l.toPolynomialInvocation = r.invocation
	}
	return a, e
}
func (l *monomialLocation) acknowledgeToPolynomialCreated(a Monomial) {
	s := l.toPolynomialCohort
	if s == nil {
		return
	}
	r := &s.direct
	if r.manager == nil || !r.manager.matchesToPolynomialInvocation(r) || r.invocation != l.toPolynomialInvocation || s.generated != a {
		return
	}
	s.generated = Monomial{}
	c := r.manager.core
	if c.generatedActive && c.generatedMonomial == a {
		c.generatedMonomial = Monomial{}
		c.generatedActive = false
		c.generatedPSC = 0
		c.generatedBuffer = nil
	}
	l.toPolynomialCohort = nil
	l.toPolynomialInvocation = 0
}
