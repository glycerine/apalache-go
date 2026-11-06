package polynomial

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func (m *UPolynomialManager) convertUpperSignAssertion(a, b int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if a != b {
		return m.scalarDivisionAssertionFailure(2864, "sign_upper == sign_b")
	}
	return nil
}
func (m *UPolynomialManager) convertBinaryOrderAssertion(line int32, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	less, e := bqm.Less(a, b)
	if e != nil {
		return e
	}
	expression := "bqm.lt(c, d)"
	if line == 2892 {
		expression = "bqm.lt(c, lower)"
	}
	if line == 2893 {
		expression = "bqm.lt(lower, upper)"
	}
	if !less {
		return m.scalarDivisionAssertionFailure(line, expression)
	}
	return nil
}

// Source: upolynomial.cpp:2913. Both actual polynomial evaluations are inside
// the runtime gate; disabled assertions create no evaluation child.
func (m *UPolynomialManager) convertFinalSignAssertion(v UPolynomialView, c, d *unumeral.BinaryRational) error {
	return m.convertFinalSignAssertionWithReceipt(v, c, d, nil)
}
func (m *UPolynomialManager) convertFinalSignAssertionWithReceipt(v UPolynomialView, c, d *unumeral.BinaryRational, receipt *upolyBinaryEvaluationInvocationReceipt) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	sc, e := m.evalSignBinaryWithNegScopeAndReceipt(v, c, nil, receipt)
	if e != nil {
		return e
	}
	sd, e := m.evalSignBinaryWithNegScopeAndReceipt(v, d, nil, receipt)
	if e != nil {
		return e
	}
	if sc != -sd {
		return m.scalarDivisionAssertionFailure(2913, "eval_sign_at(sz, p, c) == -eval_sign_at(sz, p, d)")
	}
	return nil
}
