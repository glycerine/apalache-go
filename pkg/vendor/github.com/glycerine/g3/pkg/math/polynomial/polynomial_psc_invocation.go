package polynomial

// Source: src/math/polynomial/polynomial.cpp:5751-5831,5897-5902.

type polynomialPSCInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialPSCScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesPSCInvocation(r *polynomialPSCInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.pscLocal && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (m *MultivariatePolynomialCore) pscInvocationSourceCause(r *polynomialPSCInvocationReceipt) error {
	if !m.matchesPSCInvocation(r) {
		return nil
	}
	return r.frame.sourceCause
}
func (m *MultivariatePolynomialCore) cleanupPSCInvocation(r *polynomialPSCInvocationReceipt, abort bool) error {
	if !m.matchesPSCInvocation(r) {
		return nil
	}
	return m.finishPSCPhase(true, abort)
}
func (m *MultivariatePolynomialCore) pscChildSourceCause(s *polynomialPSCScope) error {
	switch s.phase {
	case 1:
		return m.coefficientInvocationSourceCause(&s.coefficient)
	case 2:
		return m.powerInvocationSourceCause(&s.power)
	case 3:
		return m.negInvocationSourceCause(&s.negative)
	case 4:
		return m.pseudoInvocationSourceCause(&s.pseudo)
	case 5:
		return m.lazardInvocationSourceCause(&s.lazard)
	case 6:
		return m.pscNextInvocationSourceCause(&s.next)
	}
	return nil
}
func (m *MultivariatePolynomialCore) pscChildren(s *polynomialPSCScope, abort bool) error {
	if e := m.cleanupPSCNextInvocation(&s.next, abort); e != nil {
		return e
	}
	if e := m.cleanupLazardInvocation(&s.lazard, abort); e != nil {
		return e
	}
	if e := m.cleanupCoefficientInvocation(&s.coefficient, abort); e != nil {
		return e
	}
	if e := m.cleanupPseudoInvocation(&s.pseudo, abort); e != nil {
		return e
	}
	if e := m.cleanupNegInvocation(&s.negative, abort); e != nil {
		return e
	}
	if e := m.cleanupPowerInvocation(&s.power, abort); e != nil {
		return e
	}
	if abort {
		if e := m.cleanupFactoryInvocation(&s.factory, true); e != nil {
			return e
		}
		if e := m.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
			return e
		}
		return nil
	}
	return nil
}
func (m *MultivariatePolynomialCore) pscCoefficient(s *polynomialPSCScope, p Polynomial, x, k uint32) (Polynomial, error) {
	s.phase = 1
	return m.coeffWithReceipts(p, x, k, &s.coefficient, &s.append, &s.factory)
}
func (m *MultivariatePolynomialCore) pscLeading(s *polynomialPSCScope, p Polynomial, x uint32) (Polynomial, error) {
	d, e := m.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	return m.pscCoefficient(s, p, x, d)
}
func (m *MultivariatePolynomialCore) pscPower(s *polynomialPSCScope, p Polynomial, k uint32, r *polynomialReference) error {
	s.phase = 2
	return m.powerIntoWithReceipt(p, k, r, &s.power)
}
func (m *MultivariatePolynomialCore) pscNeg(s *polynomialPSCScope, p Polynomial) (Polynomial, error) {
	s.phase = 3
	return m.negWithReceipt(p, &s.negative)
}
func (m *MultivariatePolynomialCore) pscPseudo(s *polynomialPSCScope, p, q Polynomial, x uint32, r *PolynomialRef) error {
	s.phase = 4
	return m.exactPseudoRemainderWithReceipt(p, q, x, r, &s.pseudo)
}
func (m *MultivariatePolynomialCore) pscLazard(s *polynomialPSCScope, d uint32, y, p Polynomial, x uint32, r *polynomialReference) error {
	s.phase = 5
	return m.seLazardWithReceipt(d, y, p, x, r, &s.lazard)
}
func (m *MultivariatePolynomialCore) pscNextCall(s *polynomialPSCScope, d, e uint32, a, b, c, den Polynomial, x uint32, r *polynomialReference) error {
	s.phase = 6
	return m.optimizedSe1WithReceipt(d, e, a, b, c, den, x, r, &s.next)
}
