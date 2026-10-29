package polynomial

// Source: polynomial.cpp:3541-4506, concrete reached GCD child calls.

func (m *MultivariatePolynomialManager) gcdDirectChildrenPending(s *polynomialGCDFunctionScope) bool {
	c := m.core
	return c.substitutionInvocationPending(&s.directSubstitution) || m.matchesMonicInvocation(&s.directMonic) || c.matchesArithmeticInvocation(&s.directArithmetic) || c.matchesArithmeticInvocation(&s.directConstant) || c.matchesNegInvocation(&s.directNeg) || c.matchesCoefficientInvocation(&s.directCoefficient) || c.cheap.matchesAppendInvocation(&s.directAppend) || c.matchesFactoryInvocation(&s.directFactory)
}
func (m *MultivariatePolynomialManager) cleanupGCDDirectChildren(s *polynomialGCDFunctionScope, abort bool) error {
	c := m.core
	if e := c.cleanupSubstitutionInvocation(&s.directSubstitution, abort); e != nil {
		return e
	}
	if e := m.cleanupMonicInvocation(&s.directMonic, abort); e != nil {
		return e
	}
	if e := c.cleanupArithmeticInvocation(&s.directArithmetic, abort); e != nil {
		return e
	}
	if e := c.cleanupArithmeticInvocation(&s.directConstant, abort); e != nil {
		return e
	}
	if e := c.cleanupNegInvocation(&s.directNeg, abort); e != nil {
		return e
	}
	if e := c.cleanupFactoryInvocation(&s.directFactory, abort); e != nil {
		return e
	}
	if e := c.cleanupCoefficientInvocation(&s.directCoefficient, abort); e != nil {
		return e
	}
	return c.cheap.cleanupAppendInvocation(&s.directAppend, abort)
}
func (m *MultivariatePolynomialManager) gcdLeadingCoefficient(s *polynomialGCDFunctionScope, p Polynomial, x uint32) (Polynomial, error) {
	c := m.core
	d, e := c.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	result, e := c.coeffWithReceipts(p, x, d, &s.directCoefficient, &s.directAppend, &s.directFactory)
	if e != nil {
		return Polynomial{}, e
	}
	if e = c.transferFactoryResult(&s.directFactory, result); e != nil {
		return Polynomial{}, e
	}
	return result, nil
}

func (m *MultivariatePolynomialManager) gcdDirectChildSourceCause(s *polynomialGCDFunctionScope) error {
	c := m.core
	if e := c.substitutionInvocationSourceCause(&s.directSubstitution); e != nil {
		return e
	}
	if m.matchesMonicInvocation(&s.directMonic) && s.directMonic.frame.cleanupCause != nil {
		return s.directMonic.frame.cleanupCause
	}
	for _, e := range []error{c.arithmeticInvocationSourceCause(&s.directArithmetic), c.arithmeticInvocationSourceCause(&s.directConstant), c.negInvocationSourceCause(&s.directNeg), c.coefficientInvocationSourceCause(&s.directCoefficient)} {
		if e != nil {
			return e
		}
	}
	return nil
}
