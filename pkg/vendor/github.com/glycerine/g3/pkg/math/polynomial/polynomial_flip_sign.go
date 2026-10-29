package polynomial

// Source: src/math/polynomial/polynomial.cpp:3434-3442,7220-7222;
// src/math/polynomial/polynomial.h:426. Retained NLSAT atom-factory consumer.

// FlipSignIfLeadingMonomialNegative returns p itself for zero or a nonnegative
// graded-lex leading coefficient. Otherwise it returns an unreferenced negation,
// preserving coefficient magnitudes/content. No result reference is acquired;
// the caller establishes one when retaining the result, including an alias.
func (m *MultivariatePolynomialManager) FlipSignIfLeadingMonomialNegative(p Polynomial) (Polynomial, error) {
	if err := m.ready(); err != nil {
		return Polynomial{}, err
	}
	c := m.core
	if c.matchesNegInvocation(&m.flipLeadingSignCall) {
		return Polynomial{}, ErrPolynomialPending
	}
	zero, err := c.IsZero(p)
	if err != nil {
		return Polynomial{}, err
	}
	if zero {
		return p, nil
	}
	position, err := c.gradedLexMaxPosition(p)
	if err != nil {
		return Polynomial{}, err
	}
	if err = polynomialFlipSourceAssert(func() (bool, error) { return position != ^uint32(0), nil }); err != nil {
		return Polynomial{}, err
	}
	a, err := c.coefficientCore(p, position)
	if err != nil {
		return Polynomial{}, err
	}
	negative, err := c.modular.IsNeg(&a)
	if err != nil {
		return Polynomial{}, err
	}
	if negative {
		return c.negWithReceipt(p, &m.flipLeadingSignCall)
	}
	return p, nil
}

// CleanupFlipSignIfLeadingMonomialNegative retires only the actual negation
// accepted by this wrapper. A rejected call never grants cleanup authority over
// an older independent Neg invocation using the same arithmetic core.
func (m *MultivariatePolynomialManager) CleanupFlipSignIfLeadingMonomialNegative() error {
	if m == nil || m.self != m || m.core == nil || m.core.self != m.core {
		return ErrPolynomialOwner
	}
	return m.core.cleanupNegInvocation(&m.flipLeadingSignCall, true)
}
