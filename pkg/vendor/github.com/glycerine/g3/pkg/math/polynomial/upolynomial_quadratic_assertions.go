package polynomial

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Source: upolynomial.cpp:2983,3007. DEBUG product construction remains outside
// the runtime gate, but equality and discriminant predicates are erased.
func (m *UPolynomialManager) quadraticDiscriminantAssertion(disc *unumeral.Int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	zero, e := m.modular.IsZero(disc)
	if e != nil {
		return e
	}
	if zero {
		return m.scalarDivisionAssertionFailure(2983, "!m().is_zero(disc)")
	}
	return nil
}
func (m *UPolynomialManager) quadraticProductAssertion(product, p UPolynomialView) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	same, e := m.EqualViews(product, p)
	if e != nil {
		return e
	}
	if !same {
		return m.scalarDivisionAssertionFailure(3007, "eq(f1f2, p)")
	}
	return nil
}
