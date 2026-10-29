package polynomial

// Source: src/math/polynomial/polynomial.cpp:5588-5658.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type polynomialPSCChain2Scope struct {
	invocation                           uint64
	active, executing                    bool
	refs                                 [9]PolynomialRef
	constructed                          uint8
	n1, n2, n3, d0, d1, index            uint32
	n3Initialized                        bool
	phase                                uint8
	coefficient                          polynomialCoefficientInvocationReceipt
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	power                                polynomialPowerInvocationReceipt
	pseudo                               polynomialPseudoDivisionInvocationReceipt
	division                             polynomialExactDivInvocationReceipt
	arithmetic                           polynomialArithmeticInvocationReceipt
	neg                                  polynomialNegInvocationReceipt
	bodyCause, sourceCause, cleanupCause error
}

// PSCChain2 preserves the dormant source alternative, not optimized PSCChain dispatch.
func (m *MultivariatePolynomialManager) PSCChain2(p, q Polynomial, x uint32, out *PolynomialRefVector) (err error) {
	if e := m.ready(); e != nil {
		return e
	}
	if out == nil || out.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	s := &m.pscChain2
	if s.active || s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	*s = polynomialPSCChain2Scope{active: true, executing: true, invocation: s.invocation + 1}
	defer func() {
		s.executing = false
		s.bodyCause = err
		err = polynomialNewtonJoin(err, m.finishPSCChain2Source())
	}()
	c := m.core
	// Nine default references precede both degree reads in the source.
	for i := range s.refs {
		r := &s.refs[i]
		r.owner = c
		r.self = r
		s.constructed++
	}
	G1, G2, G3, Gh3 := &s.refs[0], &s.refs[1], &s.refs[2], &s.refs[3]
	g1, h0, hs0, h1, hs1 := &s.refs[4], &s.refs[5], &s.refs[6], &s.refs[7], &s.refs[8]
	n1, e := c.Degree(p, x)
	if e != nil {
		return e
	}
	n2, e := c.Degree(q, x)
	if e != nil {
		return e
	}
	if n1 > n2 {
		if e = G1.root.assign(c, p); e != nil {
			return e
		}
		if e = G2.root.assign(c, q); e != nil {
			return e
		}
	} else {
		if e = G1.root.assign(c, q); e != nil {
			return e
		}
		if e = G2.root.assign(c, p); e != nil {
			return e
		}
		n1, n2 = n2, n1
	}
	s.n1, s.n2 = n1, n2
	s.d0 = 0
	s.d1 = n1 - n2
	s.index = 1
	if e = out.Reset(); e != nil {
		return e
	}
	for {
		zero, e := c.IsZero(G2.root.value)
		if e != nil {
			return e
		}
		if !zero {
			s.phase = 1
			if e = c.exactPseudoRemainderWithReceipt(G1.root.value, G2.root.value, x, Gh3, &s.pseudo); e != nil {
				return e
			}
			s.n3, e = c.Degree(Gh3.root.value, x)
			if e != nil {
				return e
			}
			s.n3Initialized = true
			z, e := c.IsZero(Gh3.root.value)
			if e != nil {
				return e
			}
			if !z && s.d1%2 == 0 {
				s.phase = 2
				v, e := c.negWithReceipt(Gh3.root.value, &s.neg)
				if e != nil {
					return e
				}
				if e = Gh3.root.assign(c, v); e != nil {
					return e
				}
			}
		}
		if s.index > 1 {
			s.phase = 3
			d, e := c.Degree(G1.root.value, x)
			if e != nil {
				return e
			}
			v, e := c.coeffWithReceipts(G1.root.value, x, d, &s.coefficient, &s.append, &s.factory)
			if e != nil {
				return e
			}
			if e = g1.root.assignWithFactoryReceipt(c, v, &s.factory); e != nil {
				return e
			}
			s.phase = 4
			if e = c.powerIntoWithReceipt(g1.root.value, s.d0, &h1.root, &s.power); e != nil {
				return e
			}
			if s.index > 2 {
				s.phase = 5
				if e = c.powerIntoWithReceipt(h0.root.value, s.d0-1, &hs0.root, &s.power); e != nil {
					return e
				}
				s.phase = 6
				v, e := c.exactDivWithReceipt(h1.root.value, hs0.root.value, &s.division)
				if e != nil {
					return e
				}
				if e = h1.root.assign(c, v); e != nil {
					return e
				}
				s.phase = 7
				if e = out.Push(h1.root.value); e != nil {
					return e
				}
				if zero {
					return out.reversePSCChain2()
				}
			}
		}
		z := false
		if s.index != 1 {
			z, e = c.IsZero(Gh3.root.value)
			if e != nil {
				return e
			}
		}
		if s.index == 1 || z {
			if e = G3.root.assign(c, Gh3.root.value); e != nil {
				return e
			}
		} else {
			s.phase = 8
			if e = c.powerIntoWithReceipt(h1.root.value, s.d1, &hs1.root, &s.power); e != nil {
				return e
			}
			s.phase = 9
			v, e := c.mulWithReceipt(g1.root.value, hs1.root.value, &s.arithmetic)
			if e != nil {
				return e
			}
			if e = hs1.root.assign(c, v); e != nil {
				return e
			}
			s.phase = 10
			v, e = c.exactDivWithReceipt(Gh3.root.value, hs1.root.value, &s.division)
			if e != nil {
				return e
			}
			if e = G3.root.assign(c, v); e != nil {
				return e
			}
			if e = hs1.root.assign(c, Polynomial{}); e != nil {
				return e
			}
		}
		s.n1 = s.n2
		// Literal n3 has no initializer. Refuse only at its actual undefined read;
		// do not replace it with a mathematical zero or execute a fabricated child.
		if !s.n3Initialized {
			return umemory.ErrRuntimeUninitialized
		}
		s.n2 = s.n3
		s.d0 = s.d1
		s.d1 = s.n1 - s.n2
		if e = G1.root.assign(c, G2.root.value); e != nil {
			return e
		}
		if e = G2.root.assign(c, G3.root.value); e != nil {
			return e
		}
		if s.index > 1 {
			if e = h0.root.assign(c, h1.root.value); e != nil {
				return e
			}
		}
		s.index++
	}
}
func (v *PolynomialRefVector) reversePSCChain2() error {
	n, e := v.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n/2; i++ {
		a, e := v.state.get(v.owner, i)
		if e != nil {
			return e
		}
		b, e := v.state.get(v.owner, n-1-i)
		if e != nil {
			return e
		}
		v.state.values[i] = b
		v.state.values[n-1-i] = a
	}
	return nil
}
func (m *MultivariatePolynomialManager) pscChain2ChildSourceCause() error {
	s, c := &m.pscChain2, m.core
	// A callee failure returns immediately. Inspect only the youngest reached
	// call; completed prior children cannot acquire a later source stop.
	switch s.phase {
	case 1:
		return c.pseudoInvocationSourceCause(&s.pseudo)
	case 2:
		return c.negInvocationSourceCause(&s.neg)
	case 3:
		return c.coefficientInvocationSourceCause(&s.coefficient)
	case 4, 5, 8:
		return c.powerInvocationSourceCause(&s.power)
	case 6, 10:
		return c.exactDivInvocationSourceCause(&s.division)
	case 9:
		return c.arithmeticInvocationSourceCause(&s.arithmetic)
	}
	return nil
}
func (m *MultivariatePolynomialManager) finishPSCChain2Source() error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.pscChain2
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
	if e := m.pscChain2ChildSourceCause(); e != nil {
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
	if !m.pscChain2ChildrenPending() {
		s.retire()
	}
	return nil
}
func (m *MultivariatePolynomialManager) pscChain2ChildrenPending() bool {
	s, c := &m.pscChain2, m.core
	for i := range s.refs {
		if s.refs[i].root.assigning || s.refs[i].root.incoming != (Polynomial{}) {
			return true
		}
	}
	return c.matchesNegInvocation(&s.neg) || c.exactDivInvocationPending(&s.division) || c.matchesArithmeticInvocation(&s.arithmetic) || c.matchesPseudoInvocation(&s.pseudo) || c.matchesPowerInvocation(&s.power) || c.matchesCoefficientInvocation(&s.coefficient) || c.matchesFactoryInvocation(&s.factory)
}
func (m *MultivariatePolynomialManager) cleanupPSCChain2Children(abort bool) error {
	s, c := &m.pscChain2, m.core
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
		return nil
	}
	return nil
}
func (m *MultivariatePolynomialManager) CleanupPSCChain2() (err error) {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.pscChain2
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
	if e := m.cleanupPSCChain2Children(false); e != nil {
		return e
	}
	for s.constructed > 0 {
		if e := s.refs[s.constructed-1].root.closeValue(m.core, true); e != nil {
			return e
		}
		s.constructed--
	}

	// Only unjoined assignment INCs and raw children remain after all source refs.
	for i := len(s.refs) - 1; i >= 0; i-- {
		if e := s.refs[i].root.abort(m.core); e != nil {
			return e
		}
	}
	if e := m.cleanupPSCChain2Children(true); e != nil {
		return e
	}
	s.retire()
	return nil
}
func (s *polynomialPSCChain2Scope) retire() {
	*s = polynomialPSCChain2Scope{invocation: s.invocation, n1: s.n1, n2: s.n2, n3: s.n3, d0: s.d0, d1: s.d1, index: s.index, n3Initialized: s.n3Initialized, phase: s.phase, bodyCause: s.bodyCause, sourceCause: s.sourceCause, cleanupCause: s.cleanupCause, coefficient: s.coefficient, append: s.append, factory: s.factory, power: s.power, pseudo: s.pseudo, division: s.division, arithmetic: s.arithmetic, neg: s.neg}
}
