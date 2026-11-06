package polynomial

// Source: src/math/polynomial/polynomial.cpp:629-630,674.

func (m *MonomialManager) lexDistinctEndAssert(i, j int32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && !(i >= 0 || j >= 0) {
		if e = m.sourceAssertionFailure(629, "idx1 >= 0 || idx2 >= 0"); e != nil {
			return e
		}
	}
	enabled, e = monomialSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && !(i < 0 || j < 0) {
		return m.sourceAssertionFailure(630, "idx1 < 0  || idx2 < 0")
	}
	return nil
}
func (m *MonomialManager) lexMinimumEndAssert(d1, d2 uint32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled || d1 != d2 {
		return e
	}
	return m.sourceAssertionFailure(674, "min_var_degree1 != min_var_degree2")
}
