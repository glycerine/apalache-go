package polynomial

// Source: upolynomial.cpp:1047-1050. Six independent alias assertions precede
// the field assertion, local-vector declarations and U initialization.
func (m *UPolynomialManager) extendedGCDEntryAssertions(a, b UPolynomialView, u, v, d *upolyVectorState) error {
	for i, input := range [2]UPolynomialView{a, b} {
		for j, out := range [3]*upolyVectorState{u, v, d} {
			enabled, e := refinementSourceAssertionsEnabled()
			if e != nil {
				return e
			}
			if enabled && m.alias(input, out) {
				expression := [2][3]string{{"!is_alias(A, U)", "!is_alias(A, V)", "!is_alias(A, D)"}, {"!is_alias(B, U)", "!is_alias(B, V)", "!is_alias(B, D)"}}[i][j]
				if e := m.scalarDivisionAssertionFailure(int32(1047+i), expression); e != nil {
					return e
				}
			}
		}
	}
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	field, e := m.modular.Field()
	if e != nil {
		return e
	}
	if !field {
		return m.scalarDivisionAssertionFailure(1050, "field()")
	}
	return nil
}

// DEBUG_CODE computes BV and expected_D regardless of the runtime switch.
// Only this SASSERT equality predicate and debugger action are runtime-gated.
func (m *UPolynomialManager) extendedGCDIdentityAssertion(expected, d *upolyVectorState) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	ok, e := m.EqualViews(m.stateView(expected), m.stateView(d))
	if e != nil {
		return e
	}
	if !ok {
		return m.scalarDivisionAssertionFailure(1081, "eq(expected_D, D)")
	}
	return nil
}
