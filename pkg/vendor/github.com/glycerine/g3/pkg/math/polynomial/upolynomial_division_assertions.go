package polynomial

// Source: upolynomial.cpp:404,409,434-437,459,626,664-665,671,727-728,797,816,859,877,963,995. Bind the actual receiving numeric owner
// only during a failed source assertion's debugger actions.
func (m *UPolynomialManager) scalarDivisionAssertionFailure(line int32, expression string) error {
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, m.modular.BorrowTraceTags()); e != nil {
		return e
	}
	defer borrow.unbind()
	return refinementSourceAssertionFailure(line, expression)
}

// Four separate SASSERTs: runtime gating and predicate evaluation occur in
// source order, before d is initialized and before any output publication.
func (m *UPolynomialManager) divRemAliasAssertions(a UPolynomialView, b *upolyDivisorSpan, q, r *upolyVectorState) error {
	if !polynomialSourceDebug {
		return nil
	}
	for i := 0; i < 4; i++ {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if !enabled {
			continue
		}
		alias := false
		expression := ""
		line := int32(434)
		switch i {
		case 0:
			alias = m.alias(a, q)
			expression = "!is_alias(p1, q)"
		case 1:
			alias = m.aliasDivisor(b, q)
			expression = "!is_alias(p2, q)"
		case 2:
			alias = m.alias(a, r)
			expression = "!is_alias(p1, r)"
			line = 435
		case 3:
			alias = m.aliasDivisor(b, r)
			expression = "!is_alias(p2, r)"
			line = 435
		}
		if alias {
			if e := m.scalarDivisionAssertionFailure(line, expression); e != nil {
				return e
			}
		}
	}
	return nil
}
