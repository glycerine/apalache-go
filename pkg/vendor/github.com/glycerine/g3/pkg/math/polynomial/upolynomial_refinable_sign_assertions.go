package polynomial

// Source: upolynomial.cpp:2605,2619,2624,2643,2652,2661,2727,2734. Concrete sign operands
// are computed by the ordinary algorithm; predicates/actions are runtime gated.
func (m *UPolynomialManager) refinableSignAssertion(line int32, a, b int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	valid := false
	expression := "sign_a == -sign_b"
	switch line {
	case 2605:
		valid = a == -b
	case 2619:
		valid = a == 0 || a == -b
		expression = "sign_new_a == 0 || sign_new_a == -sign_b"
	case 2624:
		valid = a == b
		expression = "sign_new_a == sign_b"
	case 2643:
		valid = a == 0 || a == -b
		expression = "sign_new_b == 0 || sign_new_b == -sign_a"
	case 2652:
		valid = a == b
		expression = "sign_new_b == sign_a"
	case 2727:
		valid = a == b
		expression = "sign_new_a1 == sign_b1"
	case 2734:
		valid = a == b
		expression = "sign_new_b2 == sign_a2"
	case 2661:
		valid = a == 0 && b == 0
		expression = "sign_a == 0 && sign_b == 0"
	}
	if !valid {
		return m.scalarDivisionAssertionFailure(line, expression)
	}
	return nil
}
