package polynomial

// Source: upolynomial.cpp:517,519,528,568.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func (m *UPolynomialManager) remainderAliasAssertions(a, b UPolynomialView, r *upolyVectorState, line int32) error {
	for i, v := range [2]UPolynomialView{a, b} {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled && m.alias(v, r) {
			expression := "!is_alias(p1, buffer)"
			if i == 1 {
				expression = "!is_alias(p2, buffer)"
			}
			if e = refinementSourceAssertionFailure(line, expression); e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *UPolynomialManager) remainderLeadingAssertion(bn *unumeral.Int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	zero, e := m.modular.IsZero(bn)
	if e != nil {
		return e
	}
	if zero {
		return refinementSourceAssertionFailure(528, "!m().is_zero(b_n)")
	}
	return nil
}
