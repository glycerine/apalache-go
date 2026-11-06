package polynomial

// Source: src/math/polynomial/polynomial.cpp:6970-6980;
// src/math/polynomial/polynomial.h:319-329.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialVariableNumeralScope struct {
	invocation                           uint64
	active, executing, finishing         bool
	copy                                 unumeral.IntegerCopyAllocationReceipt
	normalize                            unumeral.IntegerNormalizeInvocationReceipt
	bodyCause, sourceCause, cleanupCause error
}
type polynomialVariableNumeralInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialVariableNumeralScope
	invocation uint64
}
type polynomialVariableNumeralHistory struct {
	invocation                           uint64
	copy                                 unumeral.IntegerCopyAllocationReceipt
	normalize                            unumeral.IntegerNormalizeInvocationReceipt
	bodyCause, sourceCause, cleanupCause error
}

func (c *MultivariatePolynomialCore) matchesVariableNumeralInvocation(r *polynomialVariableNumeralInvocationReceipt) bool {
	return c != nil && c.self == c && r != nil && r.manager == c && r.frame == &c.variableNumeral && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (c *MultivariatePolynomialCore) variableNumeralDiagnostic(r *polynomialVariableNumeralInvocationReceipt) (polynomialVariableNumeralHistory, bool) {
	if c.matchesVariableNumeralInvocation(r) {
		s := r.frame
		return polynomialVariableNumeralHistory{s.invocation, s.copy, s.normalize, s.bodyCause, s.sourceCause, s.cleanupCause}, true
	}
	if c == nil || c.self != c || r == nil || r.manager != c || r.frame != &c.variableNumeral || r.invocation == 0 {
		return polynomialVariableNumeralHistory{}, false
	}
	for i := range c.variableNumeralHistory {
		if c.variableNumeralHistory[i].invocation == r.invocation {
			return c.variableNumeralHistory[i], true
		}
	}
	return polynomialVariableNumeralHistory{}, false
}
func (c *MultivariatePolynomialCore) archiveVariableNumeralDiagnostic() error {
	s := &c.variableNumeral
	if s.invocation == 0 {
		return nil
	}
	c.variableNumeralHistory = append(c.variableNumeralHistory, polynomialVariableNumeralHistory{s.invocation, s.copy, s.normalize, s.bodyCause, s.sourceCause, s.cleanupCause})
	return nil
}
func (m *MultivariatePolynomialManager) IsVariableMonomial(p Monomial, v *uint32) (bool, error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	return m.core.isVariableMonomial(p, v)
}
func (c *MultivariatePolynomialCore) isVariableMonomial(p Monomial, v *uint32) (bool, error) {
	size, e := c.monomials.sizeCore(p)
	if e != nil || size != 1 {
		return false, e
	}
	degree, e := c.monomials.degreeAtCore(p, 0)
	if e != nil || degree != 1 {
		return false, e
	}
	variable, e := c.monomials.varCore(p, 0)
	if e != nil {
		return false, e
	}
	if v == nil {
		return false, ErrPolynomialOwner
	}
	*v = variable
	return true, nil
}
func (m *MultivariatePolynomialManager) IsVariable(p Polynomial, v *uint32) (bool, error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	c := m.core
	size, e := c.readWord(p, 8)
	if e != nil || size != 1 {
		return false, e
	}
	term, e := c.monomialCore(p, 0)
	if e != nil {
		return false, e
	}
	yes, e := c.isVariableMonomial(term, v)
	if e != nil || !yes {
		return false, e
	}
	a, e := c.coefficientCore(p, 0)
	if e != nil {
		return false, e
	}
	return c.modular.IsOne(&a)
}
func (m *MultivariatePolynomialManager) IsVariableNumeral(p Polynomial, v *uint32, n *unumeral.Int) (bool, error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	return m.core.isVariableNumeralWithReceipt(p, v, n, nil)
}
func (c *MultivariatePolynomialCore) isVariableNumeralWithReceipt(p Polynomial, v *uint32, n *unumeral.Int, r *polynomialVariableNumeralInvocationReceipt) (yes bool, err error) {
	if e := c.check(); e != nil {
		return false, e
	}
	if c.variableNumeral.executing || c.variableNumeral.finishing || c.callbackDepth != 0 {
		return false, ErrPolynomialPending
	}
	size, e := c.readWord(p, 8)
	if e != nil || size != 2 {
		return false, e
	}
	a, e := c.coefficientCore(p, 0)
	if e != nil {
		return false, e
	}
	one, e := c.modular.IsOne(&a)
	if e != nil || !one {
		return false, e
	}
	term, e := c.monomialCore(p, 0)
	if e != nil {
		return false, e
	}
	variable, e := c.isVariableMonomial(term, v)
	if e != nil || !variable {
		return false, e
	}
	term, e = c.monomialCore(p, 1)
	if e != nil {
		return false, e
	}
	unit, e := c.monomials.sizeCore(term)
	if e != nil || unit != 0 {
		return false, e
	}
	// Only the reached caller NUM assignment acquires adapter child identity.
	value, e := c.coefficientCore(p, 1)
	if e != nil {
		return false, e
	}
	if e = c.integer.Check(n); e != nil {
		return false, e
	}
	s := &c.variableNumeral
	if s.active || s.executing || s.finishing || s.invocation == ^uint64(0) || c.callbackDepth != 0 {
		return false, ErrPolynomialPending
	}
	if e = c.archiveVariableNumeralDiagnostic(); e != nil {
		return false, e
	}
	*s = polynomialVariableNumeralScope{invocation: s.invocation + 1, active: true, executing: true}
	if r != nil {
		*r = polynomialVariableNumeralInvocationReceipt{c, s, s.invocation}
	}
	defer func() {
		s.executing = false
		s.bodyCause = err
		s.sourceCause = nil
		err = errors.Join(err, s.sourceCause)
		if err == nil && !c.variableNumeralChildrenPending() {
			s.active = false
		}
	}()
	if e = c.modular.CopyFromOwnerTracked(n, c.integer, &value, &s.copy, &s.normalize); e != nil {
		return false, e
	}
	return true, nil
}
func (c *MultivariatePolynomialCore) variableNumeralChildrenPending() bool {
	s := &c.variableNumeral
	return c.integer.MatchesCopyAllocationInvocation(&s.copy) || c.modular.MatchesPNormalizeInvocation(&s.normalize)
}
func (c *MultivariatePolynomialCore) cleanupVariableNumeralInvocation(r *polynomialVariableNumeralInvocationReceipt, abort bool) (err error) {
	if !c.matchesVariableNumeralInvocation(r) {
		return nil
	}
	s := r.frame
	if s.executing || s.finishing || c.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() {
		s.finishing = false
		if err != nil && s.cleanupCause == nil {
			s.cleanupCause = err
		}
	}()
	// Reclaim PN allocation prefixes before SET's pending copy. The caller
	// numeral is borrowed and is never destroyed here.
	if e := c.modular.CleanupPNormalizeInvocation(&s.normalize, abort); e != nil {
		return e
	}
	if !abort {
		return nil
	}
	if e := c.integer.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	if !c.variableNumeralChildrenPending() {
		s.active = false
	}
	return nil
}
func (m *MultivariatePolynomialManager) CleanupVariableNumeral() error {
	if e := m.identity(); e != nil {
		return e
	}
	c := m.core
	s := &c.variableNumeral
	r := polynomialVariableNumeralInvocationReceipt{c, s, s.invocation}
	if e := c.cleanupVariableNumeralInvocation(&r, false); e != nil {
		return e
	}
	return c.cleanupVariableNumeralInvocation(&r, true)
}
