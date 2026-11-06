package polynomial

// Source: polynomial.cpp:4018-4051, reached monomial constructor branches.

import umemory "github.com/glycerine/g3/pkg/util/memory"

func (m *MultivariatePolynomialManager) gcdZPXMonomial(s *polynomialGCDFunctionScope, term Monomial, x, k uint32, variable bool) (Monomial, error) {
	if s == nil || !s.active || m.gcdDepth == 0 || m.gcdFrames[m.gcdDepth-1] != s || m.core.callbackDepth != 0 {
		return Monomial{}, ErrPolynomialOwner
	}
	var outcome monomialFactoryOutcome
	var result Monomial
	var e error
	if variable {
		result, e = m.core.generatedVariableWithOutcome(x, k, &outcome)
	} else {
		result, e = m.core.generatedRemoveVariableWithOutcome(term, x, nil, &outcome)
	}
	if outcome.allocatorInvocation.invocation != 0 {
		s.zpxAllocator = outcome.allocatorInvocation
	}
	if e != nil && outcome.started {
		s.zpxCalleeGeneration = outcome.generation
	}
	return result, e
}
func (m *MultivariatePolynomialManager) gcdZPXMonomialPending(s *polynomialGCDFunctionScope) bool {
	return s.zpxCalleeGeneration != 0 || m.core.monomials.matchesMonomialAllocatorInvocation(&s.zpxAllocator) && s.zpxAllocator.outcome.Pending != (umemory.Block{})
}
func (m *MultivariatePolynomialManager) cleanupGCDZPXMonomial(s *polynomialGCDFunctionScope) error {
	c := m.core
	if s.zpxCalleeGeneration != 0 {
		if c.monomials.creation.generation == s.zpxCalleeGeneration && c.monomials.creation.parent == c {
			if e := c.monomials.CleanupCreation(); e != nil {
				return e
			}
		}
		s.zpxCalleeGeneration = 0
	}
	return c.monomials.cleanupMonomialAllocatorInvocation(&s.zpxAllocator, true)
}
