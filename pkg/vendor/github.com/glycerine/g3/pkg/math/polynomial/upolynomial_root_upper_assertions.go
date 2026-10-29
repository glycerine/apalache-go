package polynomial

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Source: upolynomial.cpp:1950, after the actual smaller-bound swap.
func (m *UPolynomialManager) rootUpperOrderingAssertion(r, r2 *unumeral.Int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	le, e := m.modular.LessEqual(r, r2)
	if e != nil {
		return e
	}
	if !le {
		return m.scalarDivisionAssertionFailure(1950, "m().le(r, r2)")
	}
	return nil
}
