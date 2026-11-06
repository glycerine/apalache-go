package polynomial

// Source: src/math/polynomial/polynomial.cpp:2538-2544,2481-2495.

type polynomialVariableScope struct {
	invocation, calleeGeneration, singletonGeneration uint64
	active, executing                                 bool
	generated, singleton                              Monomial
	allocator                                         monomialAllocatorInvocationReceipt
	factory                                           polynomialFactoryInvocationReceipt
	bodyCause, cleanupCause                           error
}
type polynomialVariableInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialVariableScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesVariableInvocation(r *polynomialVariableInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.variableLocal && r.invocation != 0 && r.invocation == r.frame.invocation
}

// No scoped NUM/reference exists here: one is a RAW inline numeral. Failed
// placement/local INC acquisitions are unjoined owning roots, not source DT.
func (m *MultivariatePolynomialCore) cleanupVariableInvocation(r *polynomialVariableInvocationReceipt, abort bool) (err error) {
	if !m.matchesVariableInvocation(r) || !abort {
		return nil
	}
	s := r.frame
	if s.executing {
		return ErrPolynomialPending
	}
	if !s.active {
		return nil
	}
	defer func() {
		if err != nil && s.cleanupCause == nil {
			s.cleanupCause = err
		}
	}()
	if s.calleeGeneration != 0 && m.monomials.creation.generation == s.calleeGeneration && m.monomials.creation.parent == m {
		if e := m.monomials.CleanupCreation(); e != nil {
			return e
		}
		if m.generatedCalleeGeneration == s.calleeGeneration {
			m.generatedCalleePending = false
		}
		s.calleeGeneration = 0
	}
	if e := m.monomials.cleanupMonomialAllocatorInvocation(&s.allocator, true); e != nil {
		return e
	}
	if e := m.cleanupFactoryInvocation(&s.factory, true); e != nil {
		return e
	}
	if s.singleton != (Monomial{}) {
		if !m.singletonReference || m.singletonMonomial != s.singleton || m.singletonGeneration != s.singletonGeneration {
			return ErrPolynomialPending
		}
		if e := m.monomials.DecRef(s.singleton); e != nil {
			return e
		}
		if m.pscReceipts.singleton == s.singleton {
			m.pscReceipts.singleton = Monomial{}
		}
		m.singletonReference = false
		m.singletonMonomial = Monomial{}
		s.singleton = Monomial{}
	}
	if s.generated != (Monomial{}) {
		if !m.generatedActive || m.generatedMonomial != s.generated {
			return ErrPolynomialPending
		}
		if e := m.cleanupGeneratedMonomial(); e != nil {
			return e
		}
		s.generated = Monomial{}
	}
	s.active = false
	return nil
}
