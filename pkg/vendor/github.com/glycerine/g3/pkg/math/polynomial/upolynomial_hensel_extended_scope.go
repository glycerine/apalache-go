package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1029-1105;
// src/math/polynomial/upolynomial.h:361-364.

func (c *upolyHenselCallerScope) finishExtendedSource() error {
	if c.sourceCause != nil {
		return c.sourceCause
	}
	if e := c.numericSourceCause(); e != nil {
		c.sourceCause = e
		return e
	}
	// Header RAWs are unjoined on a body exception. Only an entered explicit
	// end-DEL sequence belongs before the older scoped vector declarations.
	if c.extendedRawEndEntered && !c.extendedRawEnded {
		for c.extendedRawSource < 2 {
			if _, e := upolynomialSourceNumeralDel(c.owner.manager.config.Integer, &c.monicRaw[c.extendedRawSource]); e != nil {
				c.sourceCause = e
				return e
			}
			c.extendedRawSource++
		}
		c.extendedRawEnded = true
	}
	for c.extendedScopedSource != 0 {
		if _, e := upolynomialSourceNumeralDel(c.owner.manager.config.Integer, &c.extendedScoped[c.extendedScopedSource-1]); e != nil {
			c.sourceCause = e
			return e
		}
		c.extendedScopedSource--
	}
	if e := c.finishVectors(c.extendedDebug[:], &c.extendedDebugSource); e != nil {
		return e
	}
	return c.finishVectors(c.extended[:], &c.extendedSource)
}
func (c *upolyHenselCallerScope) ownExtended(abort bool) error {
	z := c.owner.manager.config.Integer
	for c.extendedScopedOwn != 0 {
		if e := z.CleanupRawNumeral(&c.extendedScoped[c.extendedScopedOwn-1]); e != nil {
			return e
		}
		c.extendedScopedOwn--
	}
	if c.extendedRawEndEntered || abort {
		if c.extendedRawConstructed {
			for c.extendedRawOwn < 2 {
				if e := z.CleanupRawNumeral(&c.monicRaw[c.extendedRawOwn]); e != nil {
					return e
				}
				c.extendedRawOwn++
			}
		}
	}
	if e := c.ownVectors(c.extendedDebug[:], &c.extendedDebugOwn); e != nil {
		return e
	}
	if e := c.ownVectors(c.extended[:], &c.extendedOwn); e != nil {
		return e
	}
	if abort {
		c.extendedActive = false
	}
	return nil
}
func (m *UPolynomialManager) monicLocalWithCaller(v UPolynomialView, c *upolyHenselCallerScope, context integerAllocationInvocationContext) error {
	if c == nil {
		return m.monicLocal(v)
	}
	c.extendedRawConstructed = true
	if e := m.monicWithSourceContext(v, &c.monicRaw[0], &c.monicRaw[1], context); e != nil {
		return e
	}
	c.extendedRawEndEntered = true
	for c.extendedRawSource < 2 {
		if e := c.numericSourceCause(); e != nil {
			c.sourceCause = e
			return e
		}
		if _, e := upolynomialSourceNumeralDel(m.config.Integer, &c.monicRaw[c.extendedRawSource]); e != nil {
			c.sourceCause = e
			return e
		}
		c.extendedRawSource++
	}
	c.extendedRawEnded = true
	return nil
}
func (m *UPolynomialManager) divStateWithCaller(a, b UPolynomialView, q *upolyVectorState, c *upolyHenselCallerScope, context integerAllocationInvocationContext) error {
	if c == nil {
		return m.divState(a, b, q)
	}
	var d uint32
	if e := m.divRemCoreSpanWithSourceContext(a, &upolyDivisorSpan{view: b}, &d, &m.div2, &m.div1, nil, c, context); e != nil {
		return e
	}
	if e := m.reset(&m.div1); e != nil {
		return e
	}
	*q, m.div2 = m.div2, *q
	return nil
}
func (m *UPolynomialManager) extendedGCDDiagnosticWithCaller(a, b UPolynomialView, u, v, d, au *upolyVectorState, c *upolyHenselCallerScope, context integerAllocationInvocationContext) (failure error) {
	if c == nil {
		return m.extendedGCDDiagnostic(a, b, u, v, d, au)
	}
	c.extendedDebugConstructed = 2
	c.extendedDebugSource = 2
	c.extendedDebugOwn = 2
	bv, expected := &c.extendedDebug[0], &c.extendedDebug[1]
	defer func() {
		if e := c.numericSourceCause(); e != nil {
			c.sourceCause = e
			failure = joinHenselCaller(failure, e)
			return
		}
		failure = joinHenselCaller(failure, c.finishVectors(c.extendedDebug[:], &c.extendedDebugSource))
	}()
	if e := m.mulWithSourceContext(b, m.stateView(v), bv, &c.addMul, context); e != nil {
		return e
	}
	if e := m.addSubStateWithSourceContext(m.stateView(au), m.stateView(bv), expected, false, context); e != nil {
		return e
	}
	return m.extendedGCDIdentityAssertion(expected, d)
}
func (m *UPolynomialManager) extendedGCDFinishWithCaller(u, v, d *upolyVectorState, c *upolyHenselCallerScope, context integerAllocationInvocationContext) (failure error) {
	if c == nil {
		return m.extendedGCDFinish(u, v, d)
	}
	c.extendedScopedConstructed = 2
	c.extendedScopedSource = 2
	c.extendedScopedOwn = 2
	inv, lc := &c.extendedScoped[0], &c.extendedScoped[1]
	defer func() {
		if e := c.numericSourceCause(); e != nil {
			c.sourceCause = e
			failure = joinHenselCaller(failure, e)
			return
		}
		for c.extendedScopedSource != 0 {
			if _, e := upolynomialSourceNumeralDel(m.config.Integer, &c.extendedScoped[c.extendedScopedSource-1]); e != nil {
				c.sourceCause = e
				failure = joinHenselCaller(failure, e)
				return
			}
			c.extendedScopedSource--
		}
	}()
	if e := m.monicWithSourceContext(m.stateView(d), lc, inv, context); e != nil {
		return e
	}
	if e := m.mulScalarViewWithSourceContext(m.stateView(u), inv, context); e != nil {
		return e
	}
	return m.mulScalarViewWithSourceContext(m.stateView(v), inv, context)
}
