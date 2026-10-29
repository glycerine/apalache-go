package polynomial

// Source: src/math/polynomial/polynomial.cpp:4990-4995;
// src/math/polynomial/polynomial.cpp:810-851.

func (l *monomialLocation) acknowledgePseudoCreated(a Monomial) {
	s := l.pseudoCohort
	if s == nil {
		return
	}
	r := &s.directReceipt
	c := r.manager
	if c == nil || c.self != c || r.frame != &c.pseudoDivision || r.frame != s || r.invocation == 0 || r.invocation != s.invocation || r.invocation != l.pseudoInvocation || s.generated != a {
		return
	}
	s.generated = Monomial{}
	if c.generatedActive && c.generatedMonomial == a {
		c.generatedMonomial = Monomial{}
		c.generatedActive = false
		c.generatedPSC = 0
		c.generatedBuffer = nil
	}
	l.pseudoCohort = nil
	l.pseudoInvocation = 0
}
func (m *MultivariatePolynomialCore) cleanupPseudoCreated(r *polynomialPseudoDivisionInvocationReceipt) error {
	if !m.matchesPseudoInvocation(r) {
		return nil
	}
	s := r.frame
	if s.executing {
		return ErrPolynomialPending
	}
	if s.generated != (Monomial{}) {
		old := s.generated
		e := m.monomials.CleanupValue(&s.generated)
		if s.generated == (Monomial{}) && m.generatedActive && m.generatedMonomial == old {
			m.generatedMonomial = Monomial{}
			m.generatedActive = false
			m.generatedPSC = 0
			m.generatedBuffer = nil
		}
		if e != nil {
			return e
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
