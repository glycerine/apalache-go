package polynomial

// Source: upolynomial.cpp:3067,3094,3095,3107. The constant checks are separate
// actions; the final size-and-one assertion is one short-circuited predicate.
func (m *UPolynomialManager) factorNonconstantAssertion(p *upolyVectorState) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if p.size <= 1 {
		return m.scalarDivisionAssertionFailure(3067, "!is_const(C)")
	}
	return nil
}
func (m *UPolynomialManager) factorConstantSizeAssertion(p *upolyVectorState) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if p.size != 1 {
		return m.scalarDivisionAssertionFailure(3094, "C.size() == 1")
	}
	return nil
}
func (m *UPolynomialManager) factorConstantUnitAssertion(p *upolyVectorState) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	a, e := m.read(m.stateView(p), 0)
	if e != nil {
		return e
	}
	one, e := m.modular.IsOne(a)
	if e != nil {
		return e
	}
	if !one {
		a, e = m.read(m.stateView(p), 0)
		if e != nil {
			return e
		}
		minus, e := m.modular.IsMinusOne(a)
		if e != nil {
			return e
		}
		if !minus {
			return m.scalarDivisionAssertionFailure(3095, "m().is_one(C[0]) || m().is_minus_one(C[0])")
		}
	}
	return nil
}
func (m *UPolynomialManager) factorFinalOneAssertion(p *upolyVectorState) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	one := false
	if p.size == 1 {
		a, e := m.read(m.stateView(p), 0)
		if e != nil {
			return e
		}
		one, e = m.modular.IsOne(a)
		if e != nil {
			return e
		}
	}
	if !one {
		return m.scalarDivisionAssertionFailure(3107, "A.size() == 1 && m().is_one(A[0])")
	}
	return nil
}
