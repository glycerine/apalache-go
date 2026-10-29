package polynomial

// Source: src/math/polynomial/polynomial.cpp:5694-5749; src/util/obj_ref.h:31-112.

import "errors"

type polynomialPSCNextScope struct {
	sourceGeneration                           uint64
	directReceipt                              polynomialPSCNextInvocationReceipt
	refs                                       [9]polynomialReference
	constructed, cursor                        uint8
	active, bufferConstructed, bufferDestroyed bool
	bodyCause, sourceCause, cleanupCause       error
	invocation, bufferInvocation               uint64
	executing                                  bool
	finishing                                  bool
	phase                                      uint8
	coefficient                                polynomialCoefficientInvocationReceipt
	append                                     polynomialCheapAppendInvocationReceipt
	factory                                    polynomialFactoryInvocationReceipt
	arithmetic                                 polynomialArithmeticInvocationReceipt
	division                                   polynomialExactDivInvocationReceipt
	negative                                   polynomialNegInvocationReceipt
	variable                                   polynomialVariableInvocationReceipt
}

func (m *MultivariatePolynomialCore) finishPSCNext(owning bool) error {
	return m.finishPSCNextPhase(owning, owning)
}
func (m *MultivariatePolynomialCore) finishPSCNextPhase(owning, abort bool) (err error) {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.pscNext
	if s.executing || s.finishing || m.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	defer func() {
		if err != nil {
			if owning {
				if s.cleanupCause == nil {
					s.cleanupCause = err
				}
			} else if s.sourceCause == nil {
				s.sourceCause = err
			}
		}
	}()
	if !owning {
		if e := m.pscNextChildSourceCause(s); e != nil {
			return e
		}
	} else if e := m.pscNextChildren(s, false); e != nil {
		return e
	}
	for s.cursor < s.constructed {
		i := s.constructed - s.cursor - 1
		if i < 4 && s.bufferConstructed && !s.bufferDestroyed {
			if m.pscBuffer.invocation != s.bufferInvocation {
				return ErrPolynomialOwner
			}
			if e := m.pscBuffer.finalizePhase(m, owning); e != nil {
				return e
			}
			s.bufferDestroyed = true
		}
		if e := s.refs[i].closeValue(m, owning); e != nil {
			return e
		}
		s.cursor++
	}
	if s.bufferConstructed && !s.bufferDestroyed {
		if m.pscBuffer.invocation != s.bufferInvocation {
			return ErrPolynomialOwner
		}
		if e := m.pscBuffer.finalizePhase(m, owning); e != nil {
			return e
		}
		s.bufferDestroyed = true
	}
	if abort {
		for i := s.constructed; i > 0; i-- {
			if e := s.refs[i-1].abort(m); e != nil {
				return e
			}
		}
		if s.bufferConstructed {
			if m.matchesPSCNextBuffer(s) {
				if e := m.pscBuffer.finalize(m, true); e != nil {
					return e
				}
			} else if !s.bufferDestroyed {
				return ErrPolynomialOwner
			}
		}
		if e := m.pscNextChildren(s, true); e != nil {
			return e
		}
	}
	for i := uint8(0); i < s.constructed; i++ {
		if s.refs[i].assigning || s.refs[i].incoming != (Polynomial{}) {
			return nil
		}
	}
	if m.matchesFactoryInvocation(&s.factory) || m.matchesCoefficientInvocation(&s.coefficient) || m.matchesArithmeticInvocation(&s.arithmetic) || m.exactDivInvocationPending(&s.division) || m.matchesVariableInvocation(&s.variable) && s.variable.frame.active {
		return nil
	}
	s.active = false
	return nil
}

func (m *MultivariatePolynomialCore) CleanupPSCNext() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	return m.finishPSCNext(true)
}

func (m *MultivariatePolynomialCore) optimizedSe1(d, e uint32, a, sd1, se, den Polynomial, x uint32, out *polynomialReference) error {
	return m.optimizedSe1WithReceipt(d, e, a, sd1, se, den, x, out, nil)
}
func (m *MultivariatePolynomialCore) optimizedSe1WithReceipt(d, e uint32, a, sd1, se, den Polynomial, x uint32, out *polynomialReference, receipt *polynomialPSCNextInvocationReceipt) (err error) {
	if err := polynomialPSCSourceAssert(5699, "d == degree(A, x)", func() (bool, error) {
		v, err := m.Degree(a, x)
		return d == v, err
	}); err != nil {
		return err
	}
	if err := polynomialPSCSourceAssert(5700, "e == degree(S_d_1, x)", func() (bool, error) {
		v, err := m.Degree(sd1, x)
		return e == v, err
	}); err != nil {
		return err
	}
	s, eBegin := m.beginPSCNextInvocation(receipt)
	if eBegin != nil {
		return eBegin
	}
	defer func() {
		s.executing = false
		m.pscLeave()
		s.bodyCause = err
		if z := m.finishPSCNext(false); z != nil {
			err = errors.Join(err, z)
		}
	}()
	cd1, lead, xj, tmp := &s.refs[0], &s.refs[1], &s.refs[2], &s.refs[3]
	p, z := m.pscNextLeading(s, sd1, x)
	if z != nil {
		return z
	}
	if z = cd1.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
		return z
	}
	p, z = m.pscNextLeading(s, se, x)
	if z != nil {
		return z
	}
	if z = lead.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
		return z
	}
	if z = m.pscBuffer.constructWithNextReceipt(m, &s.directReceipt); z != nil {
		return z
	}
	s.bufferConstructed = true
	s.bufferInvocation = m.pscBuffer.invocation
	h := &m.pscBuffer
	if z = xj.assign(m, m.unit); z != nil {
		return z
	}
	for j := uint32(0); j <= e-1; j++ {
		p, z = m.pscNextVariable(s, x, j)
		if z != nil {
			return z
		}
		if z = xj.assign(m, p); z != nil {
			return z
		}
		p, z = m.pscNextMul(s, lead.value, xj.value)
		if z != nil {
			return z
		}
		if z = h.pushWithNextReceipt(m, p, &s.directReceipt); z != nil {
			return z
		}
	}
	if err := polynomialPSCSourceAssert(5711, "H.size() == e", func() (bool, error) {
		return uint32(len(h.values)) == e, nil
	}); err != nil {
		return err
	}
	p, z = m.pscNextVariable(s, x, e)
	if z != nil {
		return z
	}
	if z = xj.assign(m, p); z != nil {
		return z
	}
	p, z = m.pscNextMul(s, lead.value, xj.value)
	if z != nil {
		return z
	}
	if z = xj.assign(m, p); z != nil {
		return z
	}
	p, z = m.pscNextSub(s, xj.value, se)
	if z != nil {
		return z
	}
	if z = h.pushWithNextReceipt(m, p, &s.directReceipt); z != nil {
		return z
	}
	if err := polynomialPSCSourceAssert(5716, "H.size() == e+1", func() (bool, error) {
		return uint32(len(h.values)) == e+1, nil
	}); err != nil {
		return err
	}
	s.constructed = 7
	xpol, xh, xhe := &s.refs[4], &s.refs[5], &s.refs[6]
	p, z = m.pscNextVariable(s, x, 1)
	if z != nil {
		return z
	}
	if z = xpol.assign(m, p); z != nil {
		return z
	}
	for j := e + 1; j <= d-1; j++ {
		previous, z := h.get(m, j-1)
		if z != nil {
			return z
		}
		p, z = m.pscNextMul(s, xpol.value, previous)
		if z != nil {
			return z
		}
		if z = xh.assign(m, p); z != nil {
			return z
		}
		p, z = m.pscNextCoefficient(s, xh.value, x, e)
		if z != nil {
			return z
		}
		if z = xhe.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
			return z
		}
		p, z = m.pscNextMul(s, xhe.value, sd1)
		if z != nil {
			return z
		}
		if z = tmp.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
			return z
		}
		p, z = m.pscNextDivide(s, tmp.value, cd1.value)
		if z != nil {
			return z
		}
		if z = tmp.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
			return z
		}
		p, z = m.pscNextSub(s, xh.value, tmp.value)
		if z != nil {
			return z
		}
		if z = h.pushWithNextReceipt(m, p, &s.directReceipt); z != nil {
			return z
		}
	}
	if err := polynomialPSCSourceAssert(5727, "H.size() == d", func() (bool, error) {
		return uint32(len(h.values)) == d, nil
	}); err != nil {
		return err
	}
	s.constructed = 8
	D := &s.refs[7]
	if z = D.assign(m, m.zero); z != nil {
		return z
	}
	for j := uint32(0); j < d; j++ {
		p, z = m.pscNextCoefficient(s, a, x, j)
		if z != nil {
			return z
		}
		if z = tmp.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
			return z
		}
		v, z := h.get(m, j)
		if z != nil {
			return z
		}
		p, z = m.pscNextMul(s, tmp.value, v)
		if z != nil {
			return z
		}
		if z = tmp.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
			return z
		}
		p, z = m.pscNextAdd(s, D.value, tmp.value)
		if z != nil {
			return z
		}
		if z = D.assign(m, p); z != nil {
			return z
		}
	}
	s.constructed = 9
	lca := &s.refs[8]
	p, z = m.pscNextLeading(s, a, x)
	if z != nil {
		return z
	}
	if z = lca.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
		return z
	}
	p, z = m.pscNextDivide(s, D.value, lca.value)
	if z != nil {
		return z
	}
	if z = D.assign(m, p); z != nil {
		return z
	}
	v, z := h.get(m, d-1)
	if z != nil {
		return z
	}
	p, z = m.pscNextMul(s, xpol.value, v)
	if z != nil {
		return z
	}
	if z = xh.assign(m, p); z != nil {
		return z
	}
	p, z = m.pscNextCoefficient(s, xh.value, x, e)
	if z != nil {
		return z
	}
	if z = xhe.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
		return z
	}
	p, z = m.pscNextMul(s, xhe.value, sd1)
	if z != nil {
		return z
	}
	if z = xhe.assignWithFactoryReceipt(m, p, &s.factory); z != nil {
		return z
	}
	p, z = m.pscNextAdd(s, xh.value, D.value)
	if z != nil {
		return z
	}
	if z = out.assign(m, p); z != nil {
		return z
	}
	p, z = m.pscNextMul(s, cd1.value, out.value)
	if z != nil {
		return z
	}
	if z = out.assign(m, p); z != nil {
		return z
	}
	p, z = m.pscNextSub(s, out.value, xhe.value)
	if z != nil {
		return z
	}
	if z = out.assign(m, p); z != nil {
		return z
	}
	p, z = m.pscNextDivide(s, out.value, den)
	if z != nil {
		return z
	}
	if z = out.assign(m, p); z != nil {
		return z
	}
	if (d-e+1)%2 == 1 {
		p, z = m.pscNextNeg(s, out.value)
		if z != nil {
			return z
		}
		return out.assign(m, p)
	}
	return nil
}
