package polynomial

// Source: src/math/polynomial/upolynomial.cpp:192-215,258-356,403-423,432-526;
// src/math/polynomial/upolynomial_factorization.cpp:703-801,873-977.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *UPolynomialManager) callerSourceContext(c *upolyHenselCallerScope) (integerAllocationInvocationContext, error) {
	context := integerAllocationInvocationContext{}
	if c != nil {
		context = numericIntegerModularManager_henselCallerAllocationContext(m.modular, c)
		if context.henselCallerFrame(m.config.Integer) == nil {
			return context, ErrUPolynomialOwner
		}
	}
	return context, nil
}
func (m *UPolynomialManager) setViewWithCaller(out *upolyVectorState, a UPolynomialView, c *upolyHenselCallerScope) error {
	context, e := m.callerSourceContext(c)
	if e != nil {
		return e
	}
	return m.setViewWithSourceContext(out, a, context)
}
func (m *UPolynomialManager) mulWithCaller(a, b UPolynomialView, out *upolyVectorState, c *upolyHenselCallerScope) error {
	context, e := m.callerSourceContext(c)
	if e != nil {
		return e
	}
	return m.mulWithSourceContext(a, b, out, henselNumericAddMulChild(nil, c), context)
}
func (m *UPolynomialManager) addSubWithCaller(a, b UPolynomialView, out *upolyVectorState, sub bool, c *upolyHenselCallerScope) error {
	context, e := m.callerSourceContext(c)
	if e != nil {
		return e
	}
	return m.addSubStateWithSourceContext(a, b, out, sub, context)
}
func (m *UPolynomialManager) scalarWithCaller(v *upolyVectorState, a *unumeral.Int, c *upolyHenselCallerScope) error {
	if c == nil {
		return m.mulScalarState(v, a)
	}
	context, e := m.callerSourceContext(c)
	if e != nil {
		return e
	}
	zero, e := m.modular.IsZero(a)
	if e != nil {
		return e
	}
	if zero {
		return m.reset(v)
	}
	return m.mulScalarViewWithSourceContext(m.stateView(v), a, context)
}
func (m *UPolynomialManager) divScalarWithCaller(a UPolynomialView, b *unumeral.Int, c *upolyHenselCallerScope) error {
	context, e := m.callerSourceContext(c)
	if e != nil {
		return e
	}
	return m.divScalarWithSourceContext(a, b, context)
}
func (m *UPolynomialManager) toZPInPlaceWithCaller(a *upolyVectorState, c *upolyHenselCallerScope) error {
	context, e := m.callerSourceContext(c)
	if e != nil {
		return e
	}
	return m.toZPInPlaceWithSourceContext(a, context)
}
func (m *UPolynomialManager) divRemWithCaller(a, b UPolynomialView, d *uint32, q, r *upolyVectorState, c *upolyHenselCallerScope) error {
	context, e := m.callerSourceContext(c)
	if e != nil {
		return e
	}
	return m.divRemCoreSpanWithSourceContext(a, &upolyDivisorSpan{view: b}, d, q, r, nil, c, context)
}
func (m *UPolynomialManager) divWithCaller(a, b UPolynomialView, q *upolyVectorState, c *upolyHenselCallerScope) error {
	context, e := m.callerSourceContext(c)
	if e != nil {
		return e
	}
	return m.divStateWithCaller(a, b, q, c, context)
}
