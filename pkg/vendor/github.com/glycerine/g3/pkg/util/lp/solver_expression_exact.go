package lp

// Source: src/util/rational.h:449-482; concrete lp_solver<mpq,mpq> expressions.

import "github.com/glycerine/g3/pkg/util/numeral"

// Source rational binary expressions return a copy of the compound result
// reference. Capture these concrete calls without enclosing child getters.
func (m *RationalLPSolver) expressionProduct(tmp, out, left, right *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = etaRationalProduct(m.manager, tmp, out, left, right)
	return m.calls.after(m.manager, before, err)
}
func (m *RationalLPSolver) expressionSum(tmp, out, left, right *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = etaRationalSum(m.manager, tmp, out, left, right)
	return m.calls.after(m.manager, before, err)
}
func (m *RationalLPSolver) expressionDifference(tmp, out, left, right *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = etaRationalDifference(m.manager, tmp, out, left, right)
	return m.calls.after(m.manager, before, err)
}
