package polynomial

// Source: src/math/polynomial/polynomial.cpp:5177-5182;
// src/math/polynomial/polynomial.cpp:810-851.

func (m *MultivariatePolynomialCore) dividesCreatedQuotient(a, b Monomial, r *polynomialDividesInvocationReceipt) (Monomial, bool, error) {
	if !m.matchesDividesInvocation(r) || !r.frame.executing || m.callbackDepth != 0 {
		return Monomial{}, false, ErrPolynomialOwner
	}
	if m.generatedActive {
		return Monomial{}, false, ErrPolynomialPending
	}
	outcome := monomialFactoryOutcome{}
	value, ok, cause := m.monomials.divideCreated(a, b, &outcome)
	s := r.frame
	if outcome.allocatorInvocation.invocation != 0 {
		s.monomialAllocator = outcome.allocatorInvocation
	}
	if cause != nil {
		if outcome.started {
			m.recordGeneratedCallee(outcome)
			s.calleeGeneration = outcome.generation
		}
		return value, ok, cause
	}
	if outcome.created {
		m.generatedMonomial = value
		m.generatedActive = true
		m.generatedPSC = m.pscExecutingOwner()
		s.generated = value
		l, e := m.monomials.location(value)
		if e != nil {
			return value, ok, e
		}
		l.dividesCohort = s
		l.dividesInvocation = r.invocation
	}
	return value, ok, nil
}

// The actual RC write has completed. Only the tag installed by this created
// factory outcome identifies the unjoined token; no refcount or directory
// scan supplies authority. A foreign successful INC also consumes that token.
func (l *monomialLocation) acknowledgeDividesCreated(a Monomial) {
	s := l.dividesCohort
	if s == nil {
		return
	}
	r := &s.directReceipt
	c := r.core
	if c == nil || !c.matchesDividesInvocationHistory(r) || r.invocation != l.dividesInvocation || s.generated != a {
		return
	}
	s.generated = Monomial{}
	if c.generatedActive && c.generatedMonomial == a {
		c.generatedMonomial = Monomial{}
		c.generatedActive = false
		c.generatedPSC = 0
		c.generatedBuffer = nil
	}
	l.dividesCohort = nil
	l.dividesInvocation = 0
}

func (m *MultivariatePolynomialCore) cleanupDividesCreated(s *polynomialDividesScope) error {
	if s.generated != (Monomial{}) {
		old := s.generated
		cause := m.monomials.CleanupValue(&s.generated)
		if s.generated == (Monomial{}) && m.generatedActive && m.generatedMonomial == old {
			m.generatedMonomial = Monomial{}
			m.generatedActive = false
			m.generatedPSC = 0
			m.generatedBuffer = nil
		}
		if cause != nil {
			return cause
		}
	}
	if s.calleeGeneration != 0 {
		creation := &m.monomials.creation
		if creation.generation == s.calleeGeneration && creation.parent == m {
			if e := m.monomials.CleanupCreation(); e != nil {
				return e
			}
		}
		if m.generatedCalleeGeneration == s.calleeGeneration {
			m.generatedCalleePending = false
		}
		s.calleeGeneration = 0
	}
	return m.monomials.cleanupMonomialAllocatorInvocation(&s.monomialAllocator, true)
}
