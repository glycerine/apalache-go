package polynomial

// Source: src/math/polynomial/polynomial.cpp:5574-5583,5897-5902.

type polynomialPSCChain1Scope struct {
	invocation                           uint64
	active, executing                    bool
	size, index                          uint32
	phase                                uint8
	subchain                             polynomialSubresultantChainInvocationReceipt
	coefficient                          polynomialCoefficientInvocationReceipt
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	vector                               polynomialReferenceVectorSetReceipt
	bodyCause, sourceCause, cleanupCause error
}

// PSCChain1 is the literal dormant alternative. PSCChain's optimized dispatch
// and its compact output layout remain unchanged.
func (m *MultivariatePolynomialManager) PSCChain1(p, q Polynomial, x uint32, out *PolynomialRefVector) (err error) {
	if e := m.ready(); e != nil {
		return e
	}
	if out == nil || out.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	s := &m.pscChain1
	if s.active || s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	*s = polynomialPSCChain1Scope{active: true, executing: true, invocation: s.invocation + 1}
	defer func() {
		s.executing = false
		s.bodyCause = err
		err = polynomialNewtonJoin(err, m.finishPSCChain1Source())
	}()
	s.phase = 1
	if e := m.subresultantChainWithReceipt(p, q, x, out, &s.subchain); e != nil {
		return e
	}
	sz, e := out.Size()
	if e != nil {
		return e
	}
	s.size = sz
	// Both operands are uint32, including the source unsigned sz-1 expression.
	for i := uint32(0); i < sz-1; i++ {
		s.index = i
		s.phase = 2
		current, e := out.Get(i)
		if e != nil {
			return e
		}
		coefficient, e := m.core.coeffWithReceipts(current, x, i, &s.coefficient, &s.append, &s.factory)
		if e != nil {
			return e
		}
		s.phase = 3
		if e = out.setWithFactoryReceipt(i, coefficient, &s.vector, &s.factory); e != nil {
			return e
		}
	}
	s.index = sz - 1
	s.phase = 4
	return out.setWithReceipt(sz-1, m.core.unit, &s.vector)
}
func (m *MultivariatePolynomialManager) finishPSCChain1Source() error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.pscChain1
	if s.executing {
		return ErrPolynomialPending
	}
	// Source/history payload is read only after actual owner and phase guards.
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	// No lexical polynomial_ref is declared by this source caller. Each callee
	// already attempted its own source suffix before returning to this frame.
	if e := m.core.coefficientInvocationSourceCause(&s.coefficient); e != nil {
		s.sourceCause = e
		return e
	}
	if e := m.subresultantChainInvocationSourceCause(&s.subchain); e != nil {
		s.sourceCause = e
		return e
	}
	if !m.pscChain1ChildrenPending() {
		s.retire()
	}
	return nil
}
func (m *MultivariatePolynomialManager) pscChain1ChildrenPending() bool {
	s, c := &m.pscChain1, m.core
	return (m.matchesSubresultantChainInvocation(&s.subchain) && s.subchain.frame.active) || c.matchesCoefficientInvocation(&s.coefficient) || c.matchesFactoryInvocation(&s.factory) || (s.vector.matches() && s.vector.frame.setActive)
}
func (m *MultivariatePolynomialManager) CleanupPSCChain1() (err error) {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.pscChain1
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
	if e := m.core.cleanupCoefficientInvocation(&s.coefficient, false); e != nil {
		return e
	}
	if e := m.cleanupSubresultantChainInvocation(&s.subchain, false); e != nil {
		return e
	}
	// This source caller has no older lexical references to destroy between
	// completed child source acknowledgment and exact unjoined child abortion.
	if e := m.core.cleanupCoefficientInvocation(&s.coefficient, true); e != nil {
		return e
	}
	if e := m.cleanupSubresultantChainInvocation(&s.subchain, true); e != nil {
		return e
	}
	if e := s.vector.cleanup(); e != nil {
		return e
	}
	if e := m.core.cleanupFactoryInvocation(&s.factory, true); e != nil {
		return e
	}
	if e := m.core.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
		return e
	}
	s.retire()
	return nil
}
func (s *polynomialPSCChain1Scope) retire() {
	*s = polynomialPSCChain1Scope{invocation: s.invocation, size: s.size, index: s.index, phase: s.phase, subchain: s.subchain, coefficient: s.coefficient, append: s.append, factory: s.factory, vector: s.vector, bodyCause: s.bodyCause, sourceCause: s.sourceCause, cleanupCause: s.cleanupCause}
}
