package polynomial

// Source: src/math/polynomial/polynomial.cpp:5694-5749.

type polynomialPSCNextInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialPSCNextScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesPSCNextInvocation(r *polynomialPSCNextInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.pscNext && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (m *MultivariatePolynomialCore) pscNextInvocationSourceCause(r *polynomialPSCNextInvocationReceipt) error {
	if !m.matchesPSCNextInvocation(r) {
		return nil
	}
	return r.frame.sourceCause
}
func (m *MultivariatePolynomialCore) cleanupPSCNextInvocation(r *polynomialPSCNextInvocationReceipt, abort bool) error {
	if !m.matchesPSCNextInvocation(r) {
		return nil
	}
	return m.finishPSCNextPhase(true, abort)
}
func (m *MultivariatePolynomialCore) pscNextChildSourceCause(s *polynomialPSCNextScope) error {
	switch s.phase {
	case 1:
		return m.coefficientInvocationSourceCause(&s.coefficient)
	case 2:
		return m.arithmeticInvocationSourceCause(&s.arithmetic)
	case 3:
		return m.exactDivInvocationSourceCause(&s.division)
	case 5:
		return m.negInvocationSourceCause(&s.negative)
	}
	return nil
}
func (m *MultivariatePolynomialCore) pscNextChildren(s *polynomialPSCNextScope, abort bool) error {
	if e := m.cleanupNegInvocation(&s.negative, abort); e != nil {
		return e
	}
	if e := m.cleanupExactDivInvocation(&s.division, abort); e != nil {
		return e
	}
	if e := m.cleanupArithmeticInvocation(&s.arithmetic, abort); e != nil {
		return e
	}
	if e := m.cleanupCoefficientInvocation(&s.coefficient, abort); e != nil {
		return e
	}
	if e := m.cleanupVariableInvocation(&s.variable, abort); e != nil {
		return e
	}
	if abort {
		if e := m.cleanupFactoryInvocation(&s.factory, true); e != nil {
			return e
		}
		if e := m.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) pscNextLeading(s *polynomialPSCNextScope, p Polynomial, x uint32) (Polynomial, error) {
	d, e := m.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	return m.pscNextCoefficient(s, p, x, d)
}
func (m *MultivariatePolynomialCore) pscNextCoefficient(s *polynomialPSCNextScope, p Polynomial, x, k uint32) (Polynomial, error) {
	s.phase = 1
	return m.coeffWithReceipts(p, x, k, &s.coefficient, &s.append, &s.factory)
}
func (m *MultivariatePolynomialCore) pscNextMul(s *polynomialPSCNextScope, p, q Polynomial) (Polynomial, error) {
	s.phase = 2
	return m.mulWithReceipt(p, q, &s.arithmetic)
}
func (m *MultivariatePolynomialCore) pscNextAdd(s *polynomialPSCNextScope, p, q Polynomial) (Polynomial, error) {
	s.phase = 2
	return m.addWithReceipt(p, q, &s.arithmetic)
}
func (m *MultivariatePolynomialCore) pscNextSub(s *polynomialPSCNextScope, p, q Polynomial) (Polynomial, error) {
	s.phase = 2
	return m.subWithReceipt(p, q, &s.arithmetic)
}
func (m *MultivariatePolynomialCore) pscNextDivide(s *polynomialPSCNextScope, p, q Polynomial) (Polynomial, error) {
	s.phase = 3
	return m.exactDivWithReceipt(p, q, &s.division)
}
func (m *MultivariatePolynomialCore) pscNextVariable(s *polynomialPSCNextScope, x, k uint32) (Polynomial, error) {
	s.phase = 4
	return m.mkVariableWithReceipt(x, k, &s.variable)
}
func (m *MultivariatePolynomialCore) pscNextNeg(s *polynomialPSCNextScope, p Polynomial) (Polynomial, error) {
	s.phase = 5
	return m.negWithReceipt(p, &s.negative)
}

// This guarded source body begins four default references before its first lc.
func (m *MultivariatePolynomialCore) beginPSCNextInvocation(receipt *polynomialPSCNextInvocationReceipt) (*polynomialPSCNextScope, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	s := &m.pscNext
	if s.active || s.invocation == ^uint64(0) || m.callbackDepth != 0 {
		return nil, ErrPolynomialPending
	}
	if e := m.pscEnter(); e != nil {
		return nil, e
	}
	*s = polynomialPSCNextScope{active: true, executing: true, constructed: 4, invocation: s.invocation + 1, sourceGeneration: m.pscReceipts.generation}
	s.directReceipt = polynomialPSCNextInvocationReceipt{m, s, s.invocation}
	if receipt != nil {
		*receipt = s.directReceipt
	}
	return s, nil
}

func (m *MultivariatePolynomialCore) matchesPSCNextBuffer(s *polynomialPSCNextScope) bool {
	if m == nil || m.self != m || s != &m.pscNext {
		return false
	}
	b := &m.pscBuffer
	return b.self == b && b.owner == m && b.invocation == s.bufferInvocation && b.nextFrame == s && b.nextInvocation == s.invocation
}
