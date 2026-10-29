package numeral

// Source: src/util/mpzzp.h.

func (m *IntegerModularManager) dividesWithAllocationContext(a, b *Int, c integerAllocationInvocationContext) (bool, error) {
	f, e := m.Field()
	if e != nil {
		return false, e
	}
	if f {
		zero, e := m.IsZero(a)
		if e != nil {
			return false, e
		}
		if !zero {
			return true, nil
		}
	}
	return m.manager.dividesWithAllocationContext(a, b, c)
}

func (m *IntegerModularManager) setPSquareWithAllocationContext(context integerAllocationInvocationContext) error {
	if m != nil && m.self == m && callerBindingsExecuting(m.callerBindings) {
		return ErrNumericPending
	}
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertFinite(125); e != nil {
		return e
	}
	m.prime = false
	m.primeKnown = true
	if e := m.manager.mulWithAllocationContext(&m.P, &m.P, &m.P, context); e != nil {
		return e
	}
	return m.setupPWithAllocationContext(context)
}

func (m *IntegerModularManager) normalizeWithSourceContext(out *Int, c integerAllocationInvocationContext) error {
	if c.Caller != nil && c.Caller.hasNormalize() {
		return c.Caller.callNormalize(m, out)
	}
	return m.PNormalizeWithFactorHistory(out, nil, c)
}
