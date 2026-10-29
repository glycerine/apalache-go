package polynomial

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Source: upolynomial.cpp:2942. Evaluate both signs only after the runtime
// assertion gate, and after the ordinary source endpoint reset.
func (m *UPolynomialManager) normalizeIntervalFinalSignAssertion(v UPolynomialView, a, b *unumeral.BinaryRational) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	sa, e := m.EvalSignBinary(v, a)
	if e != nil {
		return e
	}
	sb, e := m.EvalSignBinary(v, b)
	if e != nil {
		return e
	}
	if sa != -sb {
		return m.scalarDivisionAssertionFailure(2942, "eval_sign_at(sz, p, a) == -eval_sign_at(sz, p, b)")
	}
	return nil
}

func (m *UPolynomialManager) normalizeIntervalNonzeroAssertion(a, b int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if a == 0 || b == 0 {
		return m.scalarDivisionAssertionFailure(2931, "sign_a != 0 && sign_b != 0")
	}
	return nil
}
