package polynomial

// Source: src/math/polynomial/polynomial.cpp:5356-5480.

func (m *MultivariatePolynomialManager) resultantLeadingCoefficient(s *polynomialGCDFunctionScope, p Polynomial, x uint32) (Polynomial, error) {
	c := m.core
	d, e := c.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	return c.coeffCoreWithReceipts(p, x, d, nil, &s.resultantCoefficient, &s.resultantAppend, &s.resultantFactory)
}
func (m *MultivariatePolynomialManager) resultantConcreteChildSourceCause(s *polynomialGCDFunctionScope) error {
	if e := m.core.negInvocationSourceCause(&s.resultantNeg); e != nil {
		return e
	}
	return m.core.coefficientInvocationSourceCause(&s.resultantCoefficient)
}
func (m *MultivariatePolynomialManager) resultantConcreteChildrenPending(s *polynomialGCDFunctionScope) bool {
	c := m.core
	return c.matchesNegInvocation(&s.resultantNeg) || c.matchesCoefficientInvocation(&s.resultantCoefficient) || c.matchesFactoryInvocation(&s.resultantFactory)
}
func (m *MultivariatePolynomialManager) cleanupResultantConcreteChildren(s *polynomialGCDFunctionScope, abort bool) error {
	c := m.core
	if e := c.cleanupNegInvocation(&s.resultantNeg, abort); e != nil {
		return e
	}
	if e := c.cleanupFactoryInvocation(&s.resultantFactory, abort); e != nil {
		return e
	}
	if e := c.cleanupCoefficientInvocation(&s.resultantCoefficient, abort); e != nil {
		return e
	}
	return c.cheap.cleanupAppendInvocation(&s.resultantAppend, abort)
}
