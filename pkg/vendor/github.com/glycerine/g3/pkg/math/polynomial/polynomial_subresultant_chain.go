package polynomial

// Source: src/math/polynomial/polynomial.cpp:5499-5572.

type polynomialSubresultantChainScope struct {
	invocation                           uint64
	active, executing                    bool
	refs                                 [3]PolynomialRef
	constructed                          uint8
	coefficient                          polynomialCoefficientInvocationReceipt
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	power                                polynomialPowerInvocationReceipt
	pseudo                               polynomialPseudoDivisionInvocationReceipt
	division                             polynomialExactDivInvocationReceipt
	arithmetic                           polynomialArithmeticInvocationReceipt
	neg                                  polynomialNegInvocationReceipt
	vector                               polynomialReferenceVectorSetReceipt
	bodyCause, sourceCause, cleanupCause error
}

// REMARK from polynomial.cpp:5500: the source algorithm does not work when
// the input degrees are equal. The source assertions and branch are retained;
// this alternative is distinct from the default optimized PSCChain dispatch.
type polynomialSubresultantChainInvocationReceipt struct {
	manager    *MultivariatePolynomialManager
	frame      *polynomialSubresultantChainScope
	invocation uint64
}

func (m *MultivariatePolynomialManager) matchesSubresultantChainInvocation(r *polynomialSubresultantChainInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.subresultant && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (m *MultivariatePolynomialManager) subresultantChainInvocationSourceCause(r *polynomialSubresultantChainInvocationReceipt) error {
	if !m.matchesSubresultantChainInvocation(r) {
		return nil
	}
	return r.frame.sourceCause
}
func (m *MultivariatePolynomialManager) SubresultantChain(p, q Polynomial, x uint32, out *PolynomialRefVector) error {
	return m.subresultantChainWithReceipt(p, q, x, out, nil)
}
func (m *MultivariatePolynomialManager) subresultantChainWithReceipt(p, q Polynomial, x uint32, out *PolynomialRefVector, receipt *polynomialSubresultantChainInvocationReceipt) (err error) {
	if e := m.ready(); e != nil {
		return e
	}
	if out == nil || out.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	s := &m.subresultant
	if s.active || s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	*s = polynomialSubresultantChainScope{active: true, executing: true, invocation: s.invocation + 1}
	if receipt != nil {
		*receipt = polynomialSubresultantChainInvocationReceipt{m, s, s.invocation}
	}
	defer func() {
		s.executing = false
		s.bodyCause = err
		err = polynomialNewtonJoin(err, m.finishSubresultantChainSource())
	}()
	c := m.core
	dp, e := c.Degree(p, x)
	if e != nil {
		return e
	}
	dq, e := c.Degree(q, x)
	if e != nil {
		return e
	}
	if dp < dq {
		p, q = q, p
		dp, dq = dq, dp
	}
	if polynomialSourceDebug && dp == 0 {
		return ErrPolynomialAssertion
	}
	n := dp
	if e = out.Reset(); e != nil {
		return e
	}
	if e = out.resizeZeroRange(n + 1); e != nil {
		return e
	}
	if e = out.setWithReceipt(n, p, &s.vector); e != nil {
		return e
	}
	if e = out.setWithReceipt(n-1, q, &s.vector); e != nil {
		return e
	}
	// Actual source declaration sites follow the two published input slots.
	for i := 0; i < 3; i++ {
		r := &s.refs[i]
		r.owner = c
		r.self = r
		s.constructed++
	}
	R, prem, newS := &s.refs[0], &s.refs[1], &s.refs[2]
	j := n - 1
	for j > 0 {
		if polynomialSourceDebug {
			above, e := out.Get(j + 1)
			if e != nil {
				return e
			}
			d, e := c.Degree(above, x)
			if e != nil {
				return e
			}
			if d != j+1 {
				return ErrPolynomialAssertion
			}
		}
		if j == n-1 {
			if e = R.root.assign(c, c.unit); e != nil {
				return e
			}
		} else {
			above, e := out.Get(j + 1)
			if e != nil {
				return e
			}
			v, e := c.coeffWithReceipts(above, x, j+1, &s.coefficient, &s.append, &s.factory)
			if e != nil {
				return e
			}
			if e = R.root.assignWithFactoryReceipt(c, v, &s.factory); e != nil {
				return e
			}
		}
		current, e := out.Get(j)
		if e != nil {
			return e
		}
		r, e := c.Degree(current, x)
		if e != nil {
			return e
		}
		if r == j {
			above, e := out.Get(j + 1)
			if e != nil {
				return e
			}
			current, e = out.Get(j)
			if e != nil {
				return e
			}
			if e = c.exactPseudoRemainderWithReceipt(above, current, x, prem, &s.pseudo); e != nil {
				return e
			}
			v, e := c.exactDivWithReceipt(prem.root.value, R.root.value, &s.division)
			if e != nil {
				return e
			}
			if e = newS.root.assign(c, v); e != nil {
				return e
			}
			v, e = c.exactDivWithReceipt(newS.root.value, R.root.value, &s.division)
			if e != nil {
				return e
			}
			if e = newS.root.assign(c, v); e != nil {
				return e
			}
			if e = out.setWithReceipt(j-1, newS.root.value, &s.vector); e != nil {
				return e
			}
			j--
			continue
		}
		if polynomialSourceDebug && r >= j {
			return ErrPolynomialAssertion
		}
		// Source int is signed 32-bit; the additions/subtractions occur as
		// unsigned first, then the literal casts determine the gap bounds.
		for i := int32(j - 1); i >= int32(r+1); i-- {
			if e = out.setWithReceipt(uint32(i), c.zero, &s.vector); e != nil {
				return e
			}
		}
		current, e = out.Get(j)
		if e != nil {
			return e
		}
		degree, e := c.Degree(current, x)
		if e != nil {
			return e
		}
		v, e := c.coeffWithReceipts(current, x, degree, &s.coefficient, &s.append, &s.factory)
		if e != nil {
			return e
		}
		if e = newS.root.assignWithFactoryReceipt(c, v, &s.factory); e != nil {
			return e
		}
		if e = c.powerIntoWithReceipt(newS.root.value, j-r, &newS.root, &s.power); e != nil {
			return e
		}
		current, e = out.Get(j)
		if e != nil {
			return e
		}
		v, e = c.mulWithReceipt(newS.root.value, current, &s.arithmetic)
		if e != nil {
			return e
		}
		if e = newS.root.assign(c, v); e != nil {
			return e
		}
		for i := uint32(0); i < j-r; i++ {
			v, e = c.exactDivWithReceipt(newS.root.value, R.root.value, &s.division)
			if e != nil {
				return e
			}
			if e = newS.root.assign(c, v); e != nil {
				return e
			}
		}
		if e = out.setWithReceipt(r, newS.root.value, &s.vector); e != nil {
			return e
		}
		if r > 0 {
			above, e := out.Get(j + 1)
			if e != nil {
				return e
			}
			current, e = out.Get(j)
			if e != nil {
				return e
			}
			if e = c.exactPseudoRemainderWithReceipt(above, current, x, prem, &s.pseudo); e != nil {
				return e
			}
			if e = newS.root.assign(c, prem.root.value); e != nil {
				return e
			}
			for i := uint32(0); i < j-r+2; i++ {
				v, e = c.exactDivWithReceipt(newS.root.value, R.root.value, &s.division)
				if e != nil {
					return e
				}
				if e = newS.root.assign(c, v); e != nil {
					return e
				}
			}
			if (j-r+2)%2 == 1 {
				v, e = c.negWithReceipt(newS.root.value, &s.neg)
				if e != nil {
					return e
				}
				if e = newS.root.assign(c, v); e != nil {
					return e
				}
			}
			if e = out.setWithReceipt(r-1, newS.root.value, &s.vector); e != nil {
				return e
			}
			j = r - 1
		} else {
			j = 0
		}
	}
	return nil
}
func (m *MultivariatePolynomialManager) subresultantChildSourceCause() error {
	s, c := &m.subresultant, m.core
	if e := c.negInvocationSourceCause(&s.neg); e != nil {
		return e
	}
	if e := c.exactDivInvocationSourceCause(&s.division); e != nil {
		return e
	}
	if e := c.arithmeticInvocationSourceCause(&s.arithmetic); e != nil {
		return e
	}
	if e := c.pseudoInvocationSourceCause(&s.pseudo); e != nil {
		return e
	}
	if e := c.powerInvocationSourceCause(&s.power); e != nil {
		return e
	}
	return c.coefficientInvocationSourceCause(&s.coefficient)
}
func (m *MultivariatePolynomialManager) finishSubresultantChainSource() error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.subresultant
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
	if e := m.subresultantChildSourceCause(); e != nil {
		s.sourceCause = e
		return e
	}
	for s.constructed > 0 {
		if e := s.refs[s.constructed-1].root.closeValue(m.core, false); e != nil {
			s.sourceCause = e
			return e
		}
		s.constructed--
	}
	if !m.subresultantChildrenPending() {
		s.retire()
	}
	return nil
}
func (m *MultivariatePolynomialManager) subresultantChildrenPending() bool {
	s, c := &m.subresultant, m.core
	for i := range s.refs {
		if s.refs[i].root.assigning || s.refs[i].root.incoming != (Polynomial{}) {
			return true
		}
	}
	return (s.vector.matches() && s.vector.frame.setActive) || c.matchesNegInvocation(&s.neg) || c.exactDivInvocationPending(&s.division) || c.matchesArithmeticInvocation(&s.arithmetic) || c.matchesPseudoInvocation(&s.pseudo) || c.matchesPowerInvocation(&s.power) || c.matchesCoefficientInvocation(&s.coefficient) || c.matchesFactoryInvocation(&s.factory)
}
func (m *MultivariatePolynomialManager) cleanupSubresultantChildren(abort bool) error {
	s, c := &m.subresultant, m.core
	if e := c.cleanupNegInvocation(&s.neg, abort); e != nil {
		return e
	}
	if e := c.cleanupExactDivInvocation(&s.division, abort); e != nil {
		return e
	}
	if e := c.cleanupArithmeticInvocation(&s.arithmetic, abort); e != nil {
		return e
	}
	if e := c.cleanupPseudoInvocation(&s.pseudo, abort); e != nil {
		return e
	}
	if e := c.cleanupPowerInvocation(&s.power, abort); e != nil {
		return e
	}
	if e := c.cleanupCoefficientInvocation(&s.coefficient, abort); e != nil {
		return e
	}
	if abort {
		if e := c.cleanupFactoryInvocation(&s.factory, true); e != nil {
			return e
		}
		if e := c.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
			return e
		}
		return s.vector.cleanup()
	}
	return nil
}
func (m *MultivariatePolynomialManager) CleanupSubresultantChain() error {
	if e := m.identity(); e != nil {
		return e
	}
	r := polynomialSubresultantChainInvocationReceipt{m, &m.subresultant, m.subresultant.invocation}
	if e := m.cleanupSubresultantChainInvocation(&r, false); e != nil {
		return e
	}
	return m.cleanupSubresultantChainInvocation(&r, true)
}
func (m *MultivariatePolynomialManager) cleanupSubresultantChainInvocation(r *polynomialSubresultantChainInvocationReceipt, abort bool) (err error) {
	if !m.matchesSubresultantChainInvocation(r) {
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
	if e := m.cleanupSubresultantChildren(false); e != nil {
		return e
	}
	for s.constructed > 0 {
		if e := s.refs[s.constructed-1].root.closeValue(m.core, true); e != nil {
			return e
		}
		s.constructed--
	}
	if !abort {
		return nil
	}
	// Only unjoined assignment INCs and raw children remain after all source refs.
	for i := len(s.refs) - 1; i >= 0; i-- {
		if e := s.refs[i].root.abort(m.core); e != nil {
			return e
		}
	}
	if e := m.cleanupSubresultantChildren(true); e != nil {
		return e
	}
	s.retire()
	return nil
}
func (s *polynomialSubresultantChainScope) retire() {
	*s = polynomialSubresultantChainScope{invocation: s.invocation, bodyCause: s.bodyCause, sourceCause: s.sourceCause, cleanupCause: s.cleanupCause, coefficient: s.coefficient, append: s.append, factory: s.factory, power: s.power, pseudo: s.pseudo, division: s.division, arithmetic: s.arithmetic, neg: s.neg, vector: s.vector}
}
