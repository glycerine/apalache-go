package polynomial

// Source: src/math/polynomial/polynomial.cpp:3512-3530.

func (m *MultivariatePolynomialManager) iccpConcreteChildSourceCause(s *polynomialICCPScope) error {
	c := m.core
	if e := c.exactDivInvocationSourceCause(&s.division); e != nil {
		return e
	}
	if e := c.negInvocationSourceCause(&s.neg); e != nil {
		return e
	}
	return c.coefficientInvocationSourceCause(&s.coefficient)
}
func (m *MultivariatePolynomialManager) iccpConcreteChildrenPending(s *polynomialICCPScope) bool {
	c := m.core
	// ExactDiv may finish its lexical suffix while its earlier remainder
	// buffer prelude still owns rows. Preserve that issued child as well.
	return c.matchesArithmeticInvocation(&s.division.prelude) || c.exactDivInvocationPending(&s.division) || c.matchesNegInvocation(&s.neg) || c.matchesCoefficientInvocation(&s.coefficient) || c.matchesFactoryInvocation(&s.coefficientFactory) || c.cheap.matchesAppendInvocation(&s.coefficientAppend)
}
func (m *MultivariatePolynomialManager) cleanupICCPConcreteChildren(s *polynomialICCPScope, abort bool) error {
	c := m.core
	if e := c.cleanupExactDivInvocation(&s.division, abort); e != nil {
		return e
	}
	if e := c.cleanupNegInvocation(&s.neg, abort); e != nil {
		return e
	}
	if e := c.cleanupFactoryInvocation(&s.coefficientFactory, abort); e != nil {
		return e
	}
	if e := c.cleanupCoefficientInvocation(&s.coefficient, abort); e != nil {
		return e
	}
	return c.cheap.cleanupAppendInvocation(&s.coefficientAppend, abort)
}

// Retire physical ownership without rewriting the immutable first causes or
// concrete child diagnostic identities. A new guarded begin replaces history.
func (s *polynomialICCPScope) retireICCPInvocation() {
	*s = polynomialICCPScope{
		header: s.header, invocation: s.invocation,
		bodyCause: s.bodyCause, cleanupCause: s.cleanupCause,
		coefficient: s.coefficient, coefficientAppend: s.coefficientAppend,
		coefficientFactory: s.coefficientFactory, neg: s.neg, division: s.division,
		gcdInvocation: s.gcdInvocation, contentInvocation: s.contentInvocation,
	}
}
