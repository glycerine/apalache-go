package polynomial

// Source: src/math/polynomial/polynomial.cpp:5817-5895.

import (
	"errors"
)

type polynomialPSCClassicScope struct {
	invocation                           uint64
	active, executing, finishing         bool
	refs                                 [13]PolynomialRef
	constructed, cursor, phase           uint8
	coefficient                          polynomialCoefficientInvocationReceipt
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	power                                polynomialPowerInvocationReceipt
	neg                                  polynomialNegInvocationReceipt
	pseudo                               polynomialPseudoDivisionInvocationReceipt
	division                             polynomialExactDivInvocationReceipt
	arithmetic                           polynomialArithmeticInvocationReceipt
	bodyCause, sourceCause, cleanupCause error
}
type polynomialPSCClassicInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialPSCClassicScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesPSCClassicInvocation(r *polynomialPSCClassicInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.pscClassic && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (m *MultivariatePolynomialCore) pscClassicInvocationSourceCause(r *polynomialPSCClassicInvocationReceipt) error {
	if !m.matchesPSCClassicInvocation(r) {
		return nil
	}
	return r.frame.sourceCause
}
func (m *MultivariatePolynomialCore) cleanupPSCClassicInvocation(r *polynomialPSCClassicInvocationReceipt, abort bool) error {
	if !m.matchesPSCClassicInvocation(r) {
		return nil
	}
	return m.finishPSCClassicPhase(true, abort)
}
func (m *MultivariatePolynomialCore) CleanupPSCClassic() error {
	return m.finishPSCClassicPhase(true, true)
}
func (m *MultivariatePolynomialManager) PSCChainClassic(p, q Polynomial, x uint32, out *PolynomialRefVector) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.pscChainClassicWithReceipt(p, q, x, out, nil)
}

func (m *MultivariatePolynomialCore) pscClassicCoefficient(p Polynomial, x, k uint32, r *polynomialReference) error {
	s := &m.pscClassic
	s.phase = 1
	v, e := m.coeffWithReceipts(p, x, k, &s.coefficient, &s.append, &s.factory)
	if e != nil {
		return e
	}
	return r.assignWithFactoryReceipt(m, v, &s.factory)
}
func (m *MultivariatePolynomialCore) pscClassicLeading(p Polynomial, x uint32, r *polynomialReference) error {
	d, e := m.Degree(p, x)
	if e != nil {
		return e
	}
	return m.pscClassicCoefficient(p, x, d, r)
}
func (m *MultivariatePolynomialCore) pscClassicPower(p Polynomial, k uint32, r *polynomialReference) error {
	s := &m.pscClassic
	s.phase = 2
	return m.powerIntoWithReceipt(p, k, r, &s.power)
}
func (m *MultivariatePolynomialCore) pscClassicNeg(p Polynomial, r *polynomialReference) error {
	s := &m.pscClassic
	s.phase = 3
	v, e := m.negWithReceipt(p, &s.neg)
	if e != nil {
		return e
	}
	return r.assign(m, v)
}
func (m *MultivariatePolynomialCore) pscClassicPseudo(p, q Polynomial, x uint32, r *PolynomialRef) error {
	s := &m.pscClassic
	s.phase = 4
	return m.exactPseudoRemainderWithReceipt(p, q, x, r, &s.pseudo)
}
func (m *MultivariatePolynomialCore) pscClassicMul(p, q Polynomial, r *polynomialReference) error {
	s := &m.pscClassic
	s.phase = 5
	v, e := m.mulWithReceipt(p, q, &s.arithmetic)
	if e != nil {
		return e
	}
	return r.assign(m, v)
}
func (m *MultivariatePolynomialCore) pscClassicDivide(p, q Polynomial, r *polynomialReference) error {
	s := &m.pscClassic
	s.phase = 6
	v, e := m.exactDivWithReceipt(p, q, &s.division)
	if e != nil {
		return e
	}
	return r.assign(m, v)
}
func (m *MultivariatePolynomialCore) pscClassicPush(out *PolynomialRefVector, p Polynomial) error {
	s := &m.pscClassic
	s.phase = 7
	return out.Push(p)
}
func (m *MultivariatePolynomialCore) pscClassicConstruct(i uint8) {
	s := &m.pscClassic
	r := &s.refs[i]
	*r = PolynomialRef{owner: m}
	r.self = r
	s.constructed = i + 1
}

func (m *MultivariatePolynomialCore) pscChainClassicWithReceipt(p, q Polynomial, x uint32, out *PolynomialRefVector, receipt *polynomialPSCClassicInvocationReceipt) (err error) {
	if e := m.check(); e != nil {
		return e
	}
	if out == nil || out.owner != m {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	s := &m.pscClassic
	if s.active || s.executing || s.finishing || s.invocation == ^uint64(0) || m.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	*s = polynomialPSCClassicScope{active: true, executing: true, invocation: s.invocation + 1}
	if receipt != nil {
		*receipt = polynomialPSCClassicInvocationReceipt{m, s, s.invocation}
	}
	defer func() {
		s.executing = false
		if s.bodyCause == nil && s.sourceCause == nil {
			s.bodyCause = err
		}
		err = errors.Join(err, m.finishPSCClassicPhase(false, false))
		if err == nil && !m.pscClassicPending() {
			s.active = false
		}
	}()
	if polynomialSourceDebug {
		dp, e := m.Degree(p, x)
		if e != nil {
			return e
		}
		if dp == 0 {
			return ErrPolynomialAssertion
		}
		dq, e := m.Degree(q, x)
		if e != nil {
			return e
		}
		if dq == 0 {
			return ErrPolynomialAssertion
		}
	}
	if e := out.Reset(); e != nil {
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
		err = m.pscChainClassicCore(p, q, x, out)
	} else {
		err = m.pscChainClassicCore(q, p, x, out)
	}
	if err != nil {
		return err
	}
	n, e := out.Size()
	if e != nil {
		return e
	}
	if n == 0 {
		if e = m.pscClassicPush(out, m.zero); e != nil {
			return e
		}
	}
	return out.reversePSCChain2()
}

func (m *MultivariatePolynomialCore) pscChainClassicCore(p, q Polynomial, x uint32, out *PolynomialRefVector) (err error) {
	s := &m.pscClassic
	dp, e := m.Degree(p, x)
	if e != nil {
		return e
	}
	dq, e := m.Degree(q, x)
	if e != nil {
		return e
	}
	if polynomialSourceDebug && dp < dq {
		return ErrPolynomialAssertion
	}
	for i := uint8(0); i < 12; i++ {
		m.pscClassicConstruct(i)
	}
	defer func() {
		s.executing = false
		s.bodyCause = err
		err = errors.Join(err, m.finishPSCClassicPhase(false, false))
		s.executing = true
	}()
	A, B, C, minusQ, lcQ, lcB, lcA := &s.refs[0], &s.refs[1], &s.refs[2], &s.refs[3], &s.refs[4], &s.refs[5], &s.refs[6]
	tmp1, tmp2, sd, minusB, ps := &s.refs[7], &s.refs[8], &s.refs[9], &s.refs[10], &s.refs[11]
	if e = m.pscClassicLeading(q, x, &lcQ.root); e != nil {
		return e
	}
	m.pscClassicConstruct(12)
	scale := &s.refs[12]
	if e = m.pscClassicPower(lcQ.root.value, dp-dq, &scale.root); e != nil {
		return e
	}
	if e = m.pscClassicNeg(q, &minusQ.root); e != nil {
		return e
	}
	if e = A.root.assign(m, q); e != nil {
		return e
	}
	if e = m.pscClassicPseudo(p, minusQ.root.value, x, B); e != nil {
		return e
	}
	for {
		d, e := m.Degree(A.root.value, x)
		if e != nil {
			return e
		}
		degreeB, e := m.Degree(B.root.value, x)
		if e != nil {
			return e
		}
		zero, e := m.IsZero(B.root.value)
		if e != nil {
			return e
		}
		if zero {
			return nil
		}
		if e = m.pscClassicCoefficient(B.root.value, x, d-1, &ps.root); e != nil {
			return e
		}
		zero, e = m.IsZero(ps.root.value)
		if e != nil {
			return e
		}
		if !zero {
			if e = m.pscClassicPush(out, ps.root.value); e != nil {
				return e
			}
		}
		delta := d - degreeB
		if delta > 1 {
			if e = m.pscClassicLeading(B.root.value, x, &lcB.root); e != nil {
				return e
			}
			if e = m.pscClassicPower(lcB.root.value, delta-1, &lcB.root); e != nil {
				return e
			}
			if e = m.pscClassicMul(lcB.root.value, B.root.value, &lcB.root); e != nil {
				return e
			}
			if e = m.pscClassicPower(scale.root.value, delta-1, &sd.root); e != nil {
				return e
			}
			if e = m.pscClassicDivide(lcB.root.value, sd.root.value, &C.root); e != nil {
				return e
			}
			if e = m.pscClassicMul(sd.root.value, scale.root.value, &sd.root); e != nil {
				return e
			}
			if e = m.pscClassicCoefficient(C.root.value, x, degreeB, &ps.root); e != nil {
				return e
			}
			zero, e = m.IsZero(ps.root.value)
			if e != nil {
				return e
			}
			if !zero {
				if e = m.pscClassicPush(out, ps.root.value); e != nil {
					return e
				}
			}
		} else {
			if polynomialSourceDebug && delta != 0 && delta != 1 {
				return ErrPolynomialAssertion
			}
			if e = C.root.assign(m, B.root.value); e != nil {
				return e
			}
			if e = m.pscClassicPower(scale.root.value, delta, &sd.root); e != nil {
				return e
			}
		}
		if degreeB == 0 {
			return nil
		}
		if e = m.pscClassicLeading(A.root.value, x, &lcA.root); e != nil {
			return e
		}
		if e = m.pscClassicNeg(B.root.value, &minusB.root); e != nil {
			return e
		}
		if e = m.pscClassicPseudo(A.root.value, minusB.root.value, x, tmp1); e != nil {
			return e
		}
		if e = m.pscClassicMul(lcA.root.value, sd.root.value, &tmp2.root); e != nil {
			return e
		}
		if e = m.pscClassicDivide(tmp1.root.value, tmp2.root.value, &B.root); e != nil {
			return e
		}
		if e = A.root.assign(m, C.root.value); e != nil {
			return e
		}
		if e = m.pscClassicLeading(A.root.value, x, &scale.root); e != nil {
			return e
		}
	}
}
