package polynomial

// Source: upolynomial.cpp:1610,1702. The assertion locations surround the
// actual destructive transform, rather than becoming fixed guard errors.
func (m *UPolynomialManager) reciprocalIndexAssertion(j uint32) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if j < 1 {
		return m.scalarDivisionAssertionFailure(1610, "j >= 1")
	}
	return nil
}
func (m *UPolynomialManager) binaryCompositionExponentAssertion(ki uint32) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if ki != 0 {
		return m.scalarDivisionAssertionFailure(1702, "k_i == 0")
	}
	return nil
}
