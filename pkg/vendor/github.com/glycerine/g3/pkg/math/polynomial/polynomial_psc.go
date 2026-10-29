package polynomial

// Source: src/math/polynomial/polynomial.cpp:5751-5831,5897-5902.

import (
	"errors"
)

type polynomialPSCScope struct {
	sourceGeneration                     uint64
	refs                                 [7]PolynomialRef
	constructed, cursor                  uint8
	active, executing                    bool
	finishing                            bool
	invocation                           uint64
	phase                                uint8
	bodyCause, sourceCause, cleanupCause error
	coefficient                          polynomialCoefficientInvocationReceipt
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	power                                polynomialPowerInvocationReceipt
	negative                             polynomialNegInvocationReceipt
	pseudo                               polynomialPseudoDivisionInvocationReceipt
	lazard                               polynomialLazardInvocationReceipt
	next                                 polynomialPSCNextInvocationReceipt
}

func (m *MultivariatePolynomialCore) pscConstruct(i uint8) {
	r := &m.pscLocal.refs[i]
	*r = PolynomialRef{owner: m}
	r.self = r
	m.pscLocal.constructed = i + 1
}
func (m *MultivariatePolynomialCore) finishPSC(owning bool) error {
	return m.finishPSCPhase(owning, owning)
}
func (m *MultivariatePolynomialCore) finishPSCPhase(owning, abort bool) (err error) {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.pscLocal
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
		if e := m.pscChildSourceCause(s); e != nil {
			return e
		}
	} else if e := m.pscChildren(s, false); e != nil {
		return e
	}
	for s.cursor < s.constructed {
		r := &s.refs[s.constructed-s.cursor-1].root
		if e := r.closeValue(m, owning); e != nil {
			return e
		}
		s.cursor++
	}
	if abort {
		for i := s.constructed; i > 0; i-- {
			if e := s.refs[i-1].root.abort(m); e != nil {
				return e
			}
		}
		if e := m.pscChildren(s, true); e != nil {
			return e
		}
	}
	for i := uint8(0); i < s.constructed; i++ {
		r := &s.refs[i].root
		if r.assigning || r.incoming != (Polynomial{}) {
			return nil
		}
	}
	if m.matchesFactoryInvocation(&s.factory) || m.matchesCoefficientInvocation(&s.coefficient) || m.matchesPowerInvocation(&s.power) || m.matchesPseudoInvocation(&s.pseudo) || m.matchesNegInvocation(&s.negative) || m.matchesPSCNextInvocation(&s.next) && s.next.frame.active || m.matchesLazardInvocation(&s.lazard) && s.lazard.frame.active {
		return nil
	}
	s.active = false
	return nil
}

func (m *MultivariatePolynomialManager) PSCChain(p, q Polynomial, x uint32, S *PolynomialRefVector) error {
	return m.pscChainWithReceipt(p, q, x, S, nil)
}
func (m *MultivariatePolynomialManager) pscChainWithReceipt(p, q Polynomial, x uint32, S *PolynomialRefVector, receipt *polynomialPSCInvocationReceipt) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.pscChainWithReceipt(p, q, x, S, receipt)
}

func (m *MultivariatePolynomialCore) PSCChain(p, q Polynomial, x uint32, S *PolynomialRefVector) error {
	return m.pscChainWithReceipt(p, q, x, S, nil)
}
func (m *MultivariatePolynomialCore) pscChainWithReceipt(p, q Polynomial, x uint32, S *PolynomialRefVector, receipt *polynomialPSCInvocationReceipt) error {
	if e := m.check(); e != nil {
		return e
	}
	if m.pscLocal.executing {
		return ErrPolynomialPending
	}
	if S == nil || S.owner != m {
		return ErrPolynomialOwner
	}
	if e := S.open(); e != nil {
		return e
	}
	if e := polynomialPSCSourceAssert(5805, "degree(P, x) > 0", func() (bool, error) {
		d, e := m.Degree(p, x)
		return d > 0, e
	}); e != nil {
		return e
	}
	if e := polynomialPSCSourceAssert(5806, "degree(Q, x) > 0", func() (bool, error) {
		d, e := m.Degree(q, x)
		return d > 0, e
	}); e != nil {
		return e
	}
	if e := S.Reset(); e != nil {
		return e
	}
	dp, e := m.Degree(p, x)
	if e != nil {
		return e
	}
	dq, e := m.Degree(q, x)
	if e != nil {
		return e
	}
	if dp >= dq {
		e = m.pscOptimizedCoreWithReceipt(p, q, x, S, receipt)
	} else {
		e = m.pscOptimizedCoreWithReceipt(q, p, x, S, receipt)
	}
	if e != nil {
		return e
	}
	n, e := S.Size()
	if e != nil {
		return e
	}
	if n == 0 {
		if e = S.Push(m.zero); e != nil {
			return e
		}
		n = 1
	}
	// Reverse the published chain by exchanging scalar handles in place.
	for i := uint32(0); i < n/2; i++ {
		a, e := S.state.get(m, i)
		if e != nil {
			return e
		}
		b, e := S.state.get(m, n-i-1)
		if e != nil {
			return e
		}
		S.state.values[i] = b
		S.state.values[n-i-1] = a
	}
	return nil
}

func (m *MultivariatePolynomialCore) pscOptimizedCore(p, q Polynomial, x uint32, S *PolynomialRefVector) error {
	return m.pscOptimizedCoreWithReceipt(p, q, x, S, nil)
}
func (m *MultivariatePolynomialCore) pscOptimizedCoreWithReceipt(p, q Polynomial, x uint32, S *PolynomialRefVector, receipt *polynomialPSCInvocationReceipt) (err error) {
	if e := m.tracePSCEntry(p, q); e != nil {
		return e
	}
	dp, e := m.Degree(p, x)
	if e != nil {
		return e
	}
	dq, e := m.Degree(q, x)
	if e != nil {
		return e
	}
	if e := polynomialPSCSourceAssert(5755, "degP >= degQ", func() (bool, error) {
		return dp >= dq, nil
	}); e != nil {
		return e
	}
	sc, e := m.beginPSCInvocation(receipt)
	if e != nil {
		return e
	}
	defer func() {
		sc.executing = false
		m.pscLeave()
		sc.bodyCause = err
		if e := m.finishPSC(false); e != nil {
			err = errors.Join(err, e)
		}
	}()
	a, b, c, minus, lc, ps := &sc.refs[0].root, &sc.refs[1].root, &sc.refs[2].root, &sc.refs[3].root, &sc.refs[4].root, &sc.refs[5].root
	t, e := m.pscLeading(sc, q, x)
	if e != nil {
		return e
	}
	if e = lc.assignWithFactoryReceipt(m, t, &sc.factory); e != nil {
		return e
	}
	m.pscConstruct(6)
	s := &sc.refs[6].root
	if e = m.pscPower(sc, lc.value, dp-dq, s); e != nil {
		return e
	}
	t, e = m.pscNeg(sc, q)
	if e != nil {
		return e
	}
	if e = minus.assignWithFactoryReceipt(m, t, &sc.factory); e != nil {
		return e
	}
	if e = a.assign(m, q); e != nil {
		return e
	}
	if e = m.pscPseudo(sc, p, minus.value, x, &sc.refs[1]); e != nil {
		return e
	}
	for {
		d, e := m.Degree(a.value, x)
		if e != nil {
			return e
		}
		degree, e := m.Degree(b.value, x)
		if e != nil {
			return e
		}
		zero, e := m.IsZero(b.value)
		if e != nil {
			return e
		}
		if zero {
			return nil
		}
		if e := m.tracePSCIteration(a.value, b.value, s.value, d, degree); e != nil {
			return e
		}
		t, e = m.pscCoefficient(sc, b.value, x, d-1)
		if e != nil {
			return e
		}
		if e = ps.assignWithFactoryReceipt(m, t, &sc.factory); e != nil {
			return e
		}
		zero, e = m.IsZero(ps.value)
		if e != nil {
			return e
		}
		if !zero {
			if e = S.Push(ps.value); e != nil {
				return e
			}
		}
		delta := d - degree
		if delta > 1 {
			if e = m.pscLazard(sc, d, s.value, b.value, x, c); e != nil {
				return e
			}
			t, e = m.pscCoefficient(sc, c.value, x, degree)
			if e != nil {
				return e
			}
			if e = ps.assignWithFactoryReceipt(m, t, &sc.factory); e != nil {
				return e
			}
			zero, e = m.IsZero(ps.value)
			if e != nil {
				return e
			}
			if !zero {
				if e = S.Push(ps.value); e != nil {
					return e
				}
			}
		} else {
			if e := polynomialPSCSourceAssert(5790, "delta == 0 || delta == 1", func() (bool, error) {
				return delta == 0 || delta == 1, nil
			}); e != nil {
				return e
			}
			if e = c.assign(m, b.value); e != nil {
				return e
			}
		}
		if degree == 0 {
			return nil
		}
		if e = m.pscNextCall(sc, d, degree, a.value, b.value, c.value, s.value, x, b); e != nil {
			return e
		}
		if e = a.assign(m, c.value); e != nil {
			return e
		}
		t, e = m.pscLeading(sc, a.value, x)
		if e != nil {
			return e
		}
		if e = s.assignWithFactoryReceipt(m, t, &sc.factory); e != nil {
			return e
		}
	}
}

func (m *MultivariatePolynomialCore) beginPSCInvocation(receipt *polynomialPSCInvocationReceipt) (*polynomialPSCScope, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	sc := &m.pscLocal
	if sc.active || sc.invocation == ^uint64(0) || m.callbackDepth != 0 {
		return nil, ErrPolynomialPending
	}
	if e := m.pscEnter(); e != nil {
		return nil, e
	}
	*sc = polynomialPSCScope{active: true, executing: true, invocation: sc.invocation + 1, sourceGeneration: m.pscReceipts.generation}
	if receipt != nil {
		*receipt = polynomialPSCInvocationReceipt{m, sc, sc.invocation}
	}
	for i := uint8(0); i < 6; i++ {
		m.pscConstruct(i)
	}
	return sc, nil
}
