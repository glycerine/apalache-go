package polynomial

// Source: src/math/polynomial/polynomial.cpp:5473-5500;
// src/util/obj_ref.h; src/util/scoped_numeral.h.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// This charged cold member records only reached source declarations and actual
// invoked child frames. It does not give the parent ownership of shared caches.

type polynomialDiscriminantScope struct {
	constant                             polynomialConstantCoefficientInvocationReceipt
	leadingNeg                           unumeral.IntegerNormalizeInvocationReceipt
	invocation                           uint64
	active, executing                    bool
	refs                                 [2]polynomialReference
	lc                                   unumeral.Int
	stack                                [3]uint8
	depth                                uint8
	bodyCause, sourceCause, cleanupCause error
	derivative                           polynomialDerivativeInvocationReceipt
	resultant                            polynomialGCDInvocationReceipt
	neg                                  polynomialNegInvocationReceipt
	scalar                               polynomialScalarDivInvocationReceipt
	coefficient                          polynomialCoefficientInvocationReceipt
	exact                                polynomialExactDivInvocationReceipt
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
}

func (m *MultivariatePolynomialManager) Discriminant(p Polynomial, x uint32, r *PolynomialRef) (err error) {
	if e := m.ready(); e != nil {
		return e
	}
	if r == nil || r.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	s := &m.discriminant
	if s.active || s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	*s = polynomialDiscriminantScope{active: true, executing: true, invocation: s.invocation + 1}
	// polynomial_ref p_prime is constructed before the degree test.
	s.stack[0], s.depth = 0, 1
	defer func() {
		s.executing = false
		s.bodyCause = err
		err = polynomialNewtonJoin(err, m.finishDiscriminantSource())
	}()
	c := m.core
	degree, e := c.Degree(p, x)
	if e != nil {
		return e
	}
	if degree == 0 {
		return r.root.assign(c, c.zero)
	}
	v, e := m.derivativeWithReceipt(p, x, &s.derivative)
	if e != nil {
		return e
	}
	if e = s.refs[0].assign(c, v); e != nil {
		return e
	}
	if e = m.resultantWithReceipt(p, s.refs[0].value, x, &r.root, &s.resultant); e != nil {
		return e
	}
	sign := uint64(degree)*uint64(degree-1)%4 != 0
	s.stack[s.depth], s.depth = 1, s.depth+1
	constant, e := c.constantCoefficientWithReceipt(p, x, degree, &s.lc, &s.constant)
	if e != nil {
		return e
	}
	if constant {
		if sign {
			if e = c.modular.NegWithReceipt(&s.lc, &s.leadingNeg); e != nil {
				return e
			}
		}
		v, e = c.scalarDivWithParentReceipts(r.root.value, &s.lc, &s.scalar, &s.append, &s.factory)
		if e != nil {
			return e
		}
		return r.root.assignWithFactoryReceipt(c, v, &s.factory)
	}
	if sign {
		v, e = c.negWithParentReceipts(r.root.value, &s.neg, &s.append, &s.factory)
		if e != nil {
			return e
		}
		if e = r.root.assignWithFactoryReceipt(c, v, &s.factory); e != nil {
			return e
		}
	}
	// The polynomial leading coefficient reference is younger than NUM lc.
	s.stack[s.depth], s.depth = 2, s.depth+1
	v, e = c.coeffWithReceipts(p, x, degree, &s.coefficient, &s.append, &s.factory)
	if e != nil {
		return e
	}
	if e = s.refs[1].assignWithFactoryReceipt(c, v, &s.factory); e != nil {
		return e
	}
	v, e = c.exactDivWithReceipt(r.root.value, s.refs[1].value, &s.exact)
	if e != nil {
		return e
	}
	return r.root.assign(c, v)
}

func (m *MultivariatePolynomialManager) discriminantYoungerSourceCause() error {
	s, c := &m.discriminant, m.core
	if e := c.exactDivInvocationSourceCause(&s.exact); e != nil {
		return e
	}
	if e := c.coefficientInvocationSourceCause(&s.coefficient); e != nil {
		return e
	}
	if e := c.scalarDivInvocationSourceCause(&s.scalar); e != nil {
		return e
	}

	if e := c.negInvocationSourceCause(&s.neg); e != nil {
		return e
	}
	if e := c.constantCoefficientInvocationSourceCause(&s.constant); e != nil {
		return e
	}
	if e := m.gcdInvocationSourceCause(&s.resultant); e != nil {
		return e
	}
	if e := m.derivativeInvocationSourceCause(&s.derivative); e != nil {
		return e
	}
	return nil
}
func (m *MultivariatePolynomialManager) discriminantOwnedPending() bool {
	s, c := &m.discriminant, m.core
	for i, tag := range m.derivative.orphanInvocations {
		if tag != 0 && s.derivative.manager == m && s.derivative.frame == &m.derivative && tag == s.derivative.invocation && m.derivative.orphans[i] != (Monomial{}) {
			return true
		}
	}
	return c.matchesConstantCoefficientInvocation(&s.constant) || m.matchesDerivativeInvocation(&s.derivative) || m.matchesGCDInvocation(&s.resultant) || c.matchesNegInvocation(&s.neg) || c.matchesScalarDivInvocation(&s.scalar) || c.matchesCoefficientInvocation(&s.coefficient) || c.matchesExactDivInvocation(&s.exact) || c.matchesFactoryInvocation(&s.factory) || c.cheap.matchesAppendInvocation(&s.append) || c.modular.MatchesPNormalizeInvocation(&s.leadingNeg)
}
func (m *MultivariatePolynomialManager) finishDiscriminantSource() error {
	s := &m.discriminant
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	if e := m.discriminantYoungerSourceCause(); e != nil {
		s.sourceCause = e
		return e
	}
	for s.depth != 0 {
		id := s.stack[s.depth-1]
		var e error
		if id == 1 {
			_, e = upolynomialSourceNumeralDel(m.core.integer, &s.lc)
		} else {
			i := 0
			if id == 2 {
				i = 1
			}
			e = s.refs[i].closeValue(m.core, false)
		}
		if e != nil {
			s.sourceCause = e
			return e
		}
		s.depth--
	}
	if !m.discriminantOwnedPending() {
		s.retireDiscriminantInvocation()
	}
	return nil
}

// Owning acknowledgement is separate from the sticky source stop. Child
// lexical destructors complete before the older parent declarations; retained
// unjoined roots are drained only after those declarations have completed.
func (m *MultivariatePolynomialManager) CleanupDiscriminant() (err error) {
	if e := m.identity(); e != nil {
		return e
	}
	s, c := &m.discriminant, m.core
	defer func() {
		if err != nil && s.cleanupCause == nil {
			s.cleanupCause = err
		}
	}()
	if !s.active {
		return nil
	}
	if s.executing {
		return ErrPolynomialPending
	}
	for _, f := range []func() error{
		func() error { return c.cleanupExactDivInvocation(&s.exact, false) },
		func() error { return c.cleanupCoefficientInvocation(&s.coefficient, false) },
		func() error { return c.cleanupScalarDivInvocation(&s.scalar, false) },
		func() error { return c.cleanupNegInvocation(&s.neg, false) },
		func() error { return c.modular.CleanupPNormalizeInvocation(&s.leadingNeg, false) },
		func() error { return c.cleanupConstantCoefficientInvocation(&s.constant, false) },
		func() error { return m.cleanupGCDInvocation(&s.resultant, false) },
		func() error { return m.cleanupDerivativeInvocation(&s.derivative, false) },
	} {
		if e := f(); e != nil {
			return e
		}
	}
	for s.depth != 0 {
		id := s.stack[s.depth-1]
		var e error
		if id == 1 {
			e = c.deleteNumeral(&s.lc)
		} else {
			i := 0
			if id == 2 {
				i = 1
			}
			e = s.refs[i].closeValue(c, true)
		}
		if e != nil {
			return e
		}
		s.depth--
	}
	for _, f := range []func() error{
		func() error { return c.cleanupExactDivInvocation(&s.exact, true) },
		func() error { return c.cleanupCoefficientInvocation(&s.coefficient, true) },
		func() error { return c.modular.CleanupPNormalizeInvocation(&s.leadingNeg, true) },
		func() error { return c.cleanupConstantCoefficientInvocation(&s.constant, true) },
		func() error { return c.cleanupFactoryInvocation(&s.factory, true) },
		func() error { return c.cheap.cleanupAppendInvocation(&s.append, true) },
		func() error { return c.cleanupScalarDivInvocation(&s.scalar, true) },
		func() error { return c.cleanupNegInvocation(&s.neg, true) },
		func() error { return m.cleanupGCDInvocation(&s.resultant, true) },
		func() error { return m.cleanupDerivativeInvocation(&s.derivative, true) },
	} {
		if e := f(); e != nil {
			return e
		}
	}
	s.retireDiscriminantInvocation()
	return nil
}

func (m *MultivariatePolynomialManager) DiscriminantSourceCause() error {
	if e := m.identity(); e != nil {
		return e
	}
	return m.discriminant.sourceCause
}

// Only a new actual guarded invocation replaces immutable diagnostics. Retiring
// ownership keeps no NUM/reference payload and grants no allocation authority.
func (s *polynomialDiscriminantScope) retireDiscriminantInvocation() {
	*s = polynomialDiscriminantScope{invocation: s.invocation,
		bodyCause: s.bodyCause, sourceCause: s.sourceCause, cleanupCause: s.cleanupCause,
		constant: s.constant, leadingNeg: s.leadingNeg, derivative: s.derivative,
		resultant: s.resultant, neg: s.neg, scalar: s.scalar, coefficient: s.coefficient,
		exact: s.exact, append: s.append, factory: s.factory}
}
