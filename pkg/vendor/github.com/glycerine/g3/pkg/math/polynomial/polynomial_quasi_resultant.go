package polynomial

// Source: src/math/polynomial/polynomial.cpp:5193-5272,7320-7322;
// src/math/polynomial/polynomial.h:847.

import (
	"errors"
)

type polynomialQuasiResultantScope struct {
	invocation                           uint64
	active, executing, finishing         bool
	refs                                 [3]PolynomialRef
	constructed, cursor, phase           uint8
	exponent                             uint32
	pseudo                               polynomialPseudoDivisionInvocationReceipt
	output                               *PolynomialRef
	bodyCause, sourceCause, cleanupCause error
}

// Diagnostic archives carry no numeral, polynomial, or allocation-root authority.
type polynomialQuasiResultantHistory struct {
	invocation                           uint64
	bodyCause, sourceCause, cleanupCause error
	pseudo                               polynomialPseudoDivisionInvocationReceipt
}

func (c *MultivariatePolynomialCore) archiveQuasiResultantHistory() error {
	s := &c.quasiResultant
	if s.invocation == 0 {
		return nil
	}
	c.quasiHistory = append(c.quasiHistory, polynomialQuasiResultantHistory{s.invocation, s.bodyCause, s.sourceCause, s.cleanupCause, s.pseudo})
	return nil
}

type polynomialQuasiResultantInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialQuasiResultantScope
	invocation uint64
}

func (c *MultivariatePolynomialCore) matchesQuasiResultantInvocation(r *polynomialQuasiResultantInvocationReceipt) bool {
	return c != nil && c.self == c && r != nil && r.manager == c && r.frame == &c.quasiResultant && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (c *MultivariatePolynomialCore) quasiResultantDiagnostic(r *polynomialQuasiResultantInvocationReceipt) (polynomialQuasiResultantHistory, bool) {
	if c.matchesQuasiResultantInvocation(r) {
		s := r.frame
		return polynomialQuasiResultantHistory{s.invocation, s.bodyCause, s.sourceCause, s.cleanupCause, s.pseudo}, true
	}
	if c == nil || c.self != c || r == nil || r.manager != c || r.frame != &c.quasiResultant || r.invocation == 0 {
		return polynomialQuasiResultantHistory{}, false
	}
	for i := range c.quasiHistory {
		if c.quasiHistory[i].invocation == r.invocation {
			return c.quasiHistory[i], true
		}
	}
	return polynomialQuasiResultantHistory{}, false
}
func (c *MultivariatePolynomialCore) quasiResultantInvocationSourceCause(r *polynomialQuasiResultantInvocationReceipt) error {
	h, _ := c.quasiResultantDiagnostic(r)
	return h.sourceCause
}
func (m *MultivariatePolynomialManager) QuasiResultant(p, q Polynomial, x uint32, r *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.quasiResultantWithReceipt(p, q, x, r, nil)
}
func (c *MultivariatePolynomialCore) quasiResultantWithReceipt(p, q Polynomial, x uint32, out *PolynomialRef, r *polynomialQuasiResultantInvocationReceipt) (err error) {
	if e := c.check(); e != nil {
		return e
	}
	if out == nil || out.owner != c {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	s := &c.quasiResultant
	if s.active || s.executing || s.finishing || s.invocation == ^uint64(0) || c.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if e := c.archiveQuasiResultantHistory(); e != nil {
		return e
	}
	*s = polynomialQuasiResultantScope{invocation: s.invocation + 1, active: true, executing: true, output: out}
	out.quasiFrame = s
	out.quasiInvocation = s.invocation
	if r != nil {
		*r = polynomialQuasiResultantInvocationReceipt{c, s, s.invocation}
	}
	defer func() {
		s.executing = false
		s.bodyCause = err
		err = errors.Join(err, c.finishQuasiResultantPhase(false, false))
		if err == nil && !c.quasiResultantChildrenPending() {
			s.active = false
		}
	}()
	if e := polynomialQuasiResultantSourceAssert(5243, "degree(p, x) > 0", func() (bool, error) {
		dp, e := c.Degree(p, x)
		return dp > 0, e
	}); e != nil {
		return e
	}
	if e := polynomialQuasiResultantSourceAssert(5244, "degree(q, x) > 0", func() (bool, error) {
		dq, e := c.Degree(q, x)
		return dq > 0, e
	}); e != nil {
		return e
	}
	// All three source reference declarations precede the degree swap.
	for i := range s.refs {
		s.refs[i].owner = c
		s.refs[i].self = &s.refs[i]
		s.constructed++
	}
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
	}
	h0, h1, h2 := &s.refs[0], &s.refs[1], &s.refs[2]
	s.phase = 1
	if e = h0.root.assign(c, p); e != nil {
		return e
	}
	if e = h1.root.assign(c, q); e != nil {
		return e
	}
	for {
		if e = polynomialQuasiResultantSourceAssert(5254, "degree(h_1, x) <= degree(h_0, x)", func() (bool, error) {
			d0, e := c.Degree(h0.root.value, x)
			if e != nil {
				return false, e
			}
			d1, e := c.Degree(h1.root.value, x)
			return d1 <= d0, e
		}); e != nil {
			return e
		}
		s.phase = 2
		if e = c.pseudoRemainderWithReceipt(h0.root.value, h1.root.value, x, &s.exponent, h2, &s.pseudo); e != nil {
			return e
		}
		if e = polynomialQuasiResultantSourceAssert(5257, "degree(h_2, x) < degree(h_1, x)", func() (bool, error) {
			d2, e := c.Degree(h2.root.value, x)
			if e != nil {
				return false, e
			}
			d1, e := c.Degree(h1.root.value, x)
			return d2 < d1, e
		}); e != nil {
			return e
		}
		degree, e := c.Degree(h2.root.value, x)
		if e != nil {
			return e
		}
		s.phase = 3
		if degree == 0 {
			return out.root.assign(c, h2.root.value)
		}
		if e = h0.root.assign(c, h1.root.value); e != nil {
			return e
		}
		if e = h1.root.assign(c, h2.root.value); e != nil {
			return e
		}
	}
}
func (c *MultivariatePolynomialCore) quasiResultantChildrenPending() bool {
	s := &c.quasiResultant
	for i := range s.refs {
		r := &s.refs[i].root
		if r.assigning || r.incoming != (Polynomial{}) {
			return true
		}
	}
	return c.matchesPseudoInvocation(&s.pseudo)
}
func (c *MultivariatePolynomialCore) finishQuasiResultantPhase(owning, abort bool) (err error) {
	if c == nil || c.self != c {
		return ErrPolynomialOwner
	}
	s := &c.quasiResultant
	if s.executing || s.finishing || c.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() {
		s.finishing = false
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
		if e := c.pseudoInvocationSourceCause(&s.pseudo); e != nil {
			return e
		}
	} else {
		if e := c.cleanupPseudoInvocation(&s.pseudo, false); e != nil {
			return e
		}
	}
	for s.cursor < s.constructed {
		if e := s.refs[s.constructed-s.cursor-1].root.closeValue(c, owning); e != nil {
			return e
		}
		s.cursor++
	}
	if abort {
		for i := int(s.constructed) - 1; i >= 0; i-- {
			if e := s.refs[i].root.abort(c); e != nil {
				return e
			}
		}
		if e := c.cleanupPseudoInvocation(&s.pseudo, true); e != nil {
			return e
		}
		if !c.quasiResultantChildrenPending() {
			s.active = false
		}
	}
	return nil
}
func (c *MultivariatePolynomialCore) cleanupQuasiResultantInvocation(r *polynomialQuasiResultantInvocationReceipt, abort bool) error {
	if !c.matchesQuasiResultantInvocation(r) {
		return nil
	}
	return c.finishQuasiResultantPhase(true, abort)
}
func (m *MultivariatePolynomialManager) CleanupQuasiResultant() error {
	if e := m.identity(); e != nil {
		return e
	}
	if e := m.core.finishQuasiResultantPhase(true, false); e != nil {
		return e
	}
	return m.core.finishQuasiResultantPhase(true, true)
}
func (r *PolynomialRef) quasiResultantPending() bool {
	if r == nil || r.self != r || r.owner == nil || r.owner.self != r.owner {
		return false
	}
	s := r.quasiFrame
	return s == &r.owner.quasiResultant && r.quasiInvocation != 0 && r.quasiInvocation == s.invocation && s.output == r && s.active
}
