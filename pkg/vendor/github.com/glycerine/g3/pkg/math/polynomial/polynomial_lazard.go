package polynomial

// Source: src/math/polynomial/polynomial.cpp:5659-5692; src/util/obj_ref.h:31-112.

import (
	"errors"
)

type polynomialLazardScope struct {
	sourceGeneration                     uint64
	x, c                                 polynomialReference
	constructed, cursor                  uint8
	active, executing                    bool
	finishing                            bool
	invocation                           uint64
	phase                                uint8
	coefficient                          polynomialCoefficientInvocationReceipt
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	arithmetic                           polynomialArithmeticInvocationReceipt
	division                             polynomialExactDivInvocationReceipt
	bodyCause, sourceCause, cleanupCause error
}
type polynomialLazardInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialLazardScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesLazardInvocation(r *polynomialLazardInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.lazardLocal && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (m *MultivariatePolynomialCore) lazardInvocationSourceCause(r *polynomialLazardInvocationReceipt) error {
	if !m.matchesLazardInvocation(r) {
		return nil
	}
	return r.frame.sourceCause
}
func (m *MultivariatePolynomialCore) cleanupLazardInvocation(r *polynomialLazardInvocationReceipt, abort bool) error {
	if !m.matchesLazardInvocation(r) {
		return nil
	}
	return m.finishLazardPhase(true, abort)
}
func (m *MultivariatePolynomialCore) lazardCoefficient(s *polynomialLazardScope, p Polynomial, x uint32) (Polynomial, error) {
	s.phase = 1
	d, e := m.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	return m.coeffWithReceipts(p, x, d, &s.coefficient, &s.append, &s.factory)
}
func (m *MultivariatePolynomialCore) lazardMul(s *polynomialLazardScope, p, q Polynomial) (Polynomial, error) {
	s.phase = 2
	return m.mulWithReceipt(p, q, &s.arithmetic)
}
func (m *MultivariatePolynomialCore) lazardDivide(s *polynomialLazardScope, p, q Polynomial) (Polynomial, error) {
	s.phase = 3
	return m.exactDivWithReceipt(p, q, &s.division)
}
func (m *MultivariatePolynomialCore) lazardChildSourceCause(s *polynomialLazardScope) error {
	switch s.phase {
	case 1:
		return m.coefficientInvocationSourceCause(&s.coefficient)
	case 2:
		return m.arithmeticInvocationSourceCause(&s.arithmetic)
	case 3:
		return m.exactDivInvocationSourceCause(&s.division)
	}
	return nil
}
func (m *MultivariatePolynomialCore) lazardChildren(s *polynomialLazardScope, abort bool) error {
	if e := m.cleanupExactDivInvocation(&s.division, abort); e != nil {
		return e
	}
	if e := m.cleanupArithmeticInvocation(&s.arithmetic, abort); e != nil {
		return e
	}
	if e := m.cleanupCoefficientInvocation(&s.coefficient, abort); e != nil {
		return e
	}
	if abort {
		if e := m.cleanupFactoryInvocation(&s.factory, true); e != nil {
			return e
		}
		return m.cheap.cleanupAppendInvocation(&s.append, true)
	}
	return nil
}
func (m *MultivariatePolynomialCore) lazardChildrenPending(s *polynomialLazardScope) bool {
	return s.x.assigning || s.c.assigning || s.x.incoming != (Polynomial{}) || s.c.incoming != (Polynomial{}) || m.exactDivInvocationPending(&s.division) || m.matchesArithmeticInvocation(&s.arithmetic) || m.matchesCoefficientInvocation(&s.coefficient) || m.matchesFactoryInvocation(&s.factory)
}
func (m *MultivariatePolynomialCore) finishLazard(owning bool) error {
	return m.finishLazardPhase(owning, owning)
}
func (m *MultivariatePolynomialCore) finishLazardPhase(owning, abort bool) (err error) {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.lazardLocal
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
		if e := m.lazardChildSourceCause(s); e != nil {
			return e
		}
	} else {
		if e := m.lazardChildren(s, false); e != nil {
			return e
		}
	}
	for s.cursor < s.constructed {
		r := &s.x
		if s.constructed-s.cursor == 2 {
			r = &s.c
		}
		if e := r.closeValue(m, owning); e != nil {
			return e
		}
		s.cursor++
	}
	if abort {
		if s.constructed == 2 {
			if e := s.c.abort(m); e != nil {
				return e
			}
		}
		if s.constructed >= 1 {
			if e := s.x.abort(m); e != nil {
				return e
			}
		}
		if e := m.lazardChildren(s, true); e != nil {
			return e
		}
	}
	if !m.lazardChildrenPending(s) {
		s.retire()
	}
	return nil
}
func (s *polynomialLazardScope) retire() {
	*s = polynomialLazardScope{constructed: s.constructed, cursor: s.cursor, invocation: s.invocation, sourceGeneration: s.sourceGeneration, phase: s.phase, coefficient: s.coefficient, append: s.append, factory: s.factory, arithmetic: s.arithmetic, division: s.division, bodyCause: s.bodyCause, sourceCause: s.sourceCause, cleanupCause: s.cleanupCause}
}
func (m *MultivariatePolynomialCore) CleanupLazard() error { return m.finishLazard(true) }
func (m *MultivariatePolynomialCore) seLazard(d uint32, y, sd1 Polynomial, x uint32, out *polynomialReference) error {
	return m.seLazardWithReceipt(d, y, sd1, x, out, nil)
}
func (m *MultivariatePolynomialCore) seLazardWithReceipt(d uint32, y, sd1 Polynomial, x uint32, out *polynomialReference, receipt *polynomialLazardInvocationReceipt) (err error) {
	degree, e := m.Degree(sd1, x)
	if e != nil {
		return e
	}
	n := d - degree - 1
	if e := m.traceLazardEntry(y, sd1, n); e != nil {
		return e
	}
	if n == 0 {
		return out.assign(m, sd1)
	}
	s, e := m.beginLazardInvocation(receipt)
	if e != nil {
		return e
	}
	defer func() {
		s.executing = false
		m.pscLeave()
		s.bodyCause = err
		if e := m.finishLazard(false); e != nil {
			err = errors.Join(err, e)
		}
	}()
	p, e := m.lazardCoefficient(s, sd1, x)
	if e != nil {
		return e
	}
	if e = s.x.assignWithFactoryReceipt(m, p, &s.factory); e != nil {
		return e
	}
	a := uint32(1)
	for v := n; v > 1; v >>= 1 {
		a <<= 1
	}
	if e := m.traceLazardPower(a); e != nil {
		return e
	}
	if e := polynomialPSCSourceAssert(5672, "a <= n", func() (bool, error) {
		return a <= n, nil
	}); e != nil {
		return e
	}
	if e := polynomialPSCSourceAssert(5673, "n < 2*a", func() (bool, error) {
		return n < 2*a, nil
	}); e != nil {
		return e
	}
	s.constructed = 2
	if e = s.c.assign(m, s.x.value); e != nil {
		return e
	}
	n -= a
	for a != 1 {
		a /= 2
		p, e = m.lazardMul(s, s.c.value, s.c.value)
		if e != nil {
			return e
		}
		if e = s.c.assign(m, p); e != nil {
			return e
		}
		p, e = m.lazardDivide(s, s.c.value, y)
		if e != nil {
			return e
		}
		if e = s.c.assign(m, p); e != nil {
			return e
		}
		if e := m.traceLazardLoop(a, s.c.value); e != nil {
			return e
		}
		if n >= a {
			p, e = m.lazardMul(s, s.c.value, s.x.value)
			if e != nil {
				return e
			}
			if e = s.c.assign(m, p); e != nil {
				return e
			}
			p, e = m.lazardDivide(s, s.c.value, y)
			if e != nil {
				return e
			}
			if e = s.c.assign(m, p); e != nil {
				return e
			}
			n -= a
			if e := m.traceLazardBranch(s.c.value); e != nil {
				return e
			}
		}
	}
	if e := m.traceLazardFinal(s.c.value, y); e != nil {
		return e
	}
	p, e = m.lazardMul(s, s.c.value, sd1)
	if e != nil {
		return e
	}
	if e = out.assign(m, p); e != nil {
		return e
	}
	p, e = m.lazardDivide(s, out.value, y)
	if e != nil {
		return e
	}
	return out.assign(m, p)
}

func (m *MultivariatePolynomialCore) beginLazardInvocation(receipt *polynomialLazardInvocationReceipt) (*polynomialLazardScope, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	s := &m.lazardLocal
	if s.active || s.invocation == ^uint64(0) || m.callbackDepth != 0 {
		return nil, ErrPolynomialPending
	}
	if e := m.pscEnter(); e != nil {
		return nil, e
	}
	*s = polynomialLazardScope{active: true, executing: true, constructed: 1, invocation: s.invocation + 1, sourceGeneration: m.pscReceipts.generation}
	if receipt != nil {
		*receipt = polynomialLazardInvocationReceipt{m, s, s.invocation}
	}
	return s, nil
}
