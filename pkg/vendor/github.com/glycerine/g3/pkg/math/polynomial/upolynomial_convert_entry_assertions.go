package polynomial

// Source: upolynomial.cpp:2820-2821. Each source assertion resolves its own
// runtime action before found_d and either scoped conversion temporary.
func (m *UPolynomialManager) convertIntervalEntryAssertions(a, b int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && (a == 0 || b == 0) {
		if e := m.scalarDivisionAssertionFailure(2820, "sign_a != 0 && sign_b != 0"); e != nil {
			return e
		}
	}
	enabled, e = refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && a != -b {
		return m.scalarDivisionAssertionFailure(2821, "sign_a == -sign_b")
	}
	return nil
}
