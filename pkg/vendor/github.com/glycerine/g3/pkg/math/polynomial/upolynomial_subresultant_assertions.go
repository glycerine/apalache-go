package polynomial

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Source: upolynomial.cpp:902,952,955. Evaluate each source predicate only
// after its runtime gate; failed actions borrow the actual numeric owner.
func (m *UPolynomialManager) subresultantDegreeAssertion(a, b *upolyVectorState) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if a.size < b.size {
		return m.scalarDivisionAssertionFailure(902, "A.size() >= B.size()")
	}
	return nil
}
func (m *UPolynomialManager) subresultantExponentAssertion(d uint32) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if d <= 1 {
		return m.scalarDivisionAssertionFailure(952, "d > 1")
	}
	return nil
}
func (m *UPolynomialManager) subresultantDivisibilityAssertion(h, aux *unumeral.Int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	divides, e := m.modular.Divides(h, aux)
	if e != nil {
		return e
	}
	if !divides {
		return m.scalarDivisionAssertionFailure(955, "m().divides(h, aux)")
	}
	return nil
}
