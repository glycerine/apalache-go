package numeral

// Source: src/util/mpz.cpp display_smt2, mpq.cpp display_smt2/display_decimal,
// and mpq.h to_rational_string/display_pp forwarding wrappers.

// appendDisplay copies static punctuation into the existing budgeted workspace.
// It preserves the complete prefix if preparation grows the byte owner.
func (m *NaturalManager) appendDisplay(offset uint32, text string) (uint32, error) {
	end, e := checkedTextEnd(offset, uint64(len(text)))
	if e != nil {
		return 0, e
	}
	if e = m.reserveText(end); e != nil {
		return 0, e
	}
	for i := 0; i < len(text); i++ {
		m.textWrite(offset+uint32(i), text[i])
	}
	return end, nil
}
func (m *IntegerManager) prepareSMT2(a *Int, decimal bool, offset uint32) (length uint32, result error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	input := a
	// Source scoped temporary is created only in the negative branch; Copy/Neg
	// precede emission of the negative SMT2 prefix.
	if a.Val < 0 {
		if e := m.beginSMT2Display(); e != nil {
			return 0, e
		}
		defer m.finishSMT2Display(&length, &result)
		tmp := &m.Smt2Display.Value
		if e := m.copySMT2Temporary(a); e != nil {
			return 0, e
		}
		if e := m.Neg(tmp); e != nil {
			return 0, e
		}
		input = tmp
		var e error
		offset, e = m.Natural.appendDisplay(offset, "(- ")
		if e != nil {
			return 0, e
		}
	}
	n, e := m.prepareInteger(input, offset)
	if e != nil {
		return 0, e
	}
	if decimal {
		n, e = m.Natural.appendDisplay(n, ".0")
		if e != nil {
			return 0, e
		}
	}
	if a.Val < 0 {
		n, e = m.Natural.appendDisplay(n, ")")
		if e != nil {
			return 0, e
		}
	}
	return n, nil
}

// FormatSMT2Into follows source expression syntax. Length excludes any C NUL;
// short buffers and predictable preparation errors leave dst unchanged.
func (m *IntegerManager) FormatSMT2Into(a *Int, decimal bool, dst []byte) (int, error) {
	n, e := m.prepareSMT2(a, decimal, 0)
	if e != nil {
		return 0, e
	}
	return m.Natural.publishText(dst, n)
}

// SMT2String allocates only the final caller-owned host string after preparing
// complete text in the numeric owner's accounted workspace.
func (m *IntegerManager) SMT2String(a *Int, decimal bool) (string, error) {
	n, e := m.prepareSMT2(a, decimal, 0)
	if e != nil {
		return "", e
	}
	return m.Natural.finalString(n)
}
func (m *RationalManager) prepareSMT2(a *Rational, decimal bool) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if isIntegerRational(a) {
		return m.Z.prepareSMT2(&a.Num, decimal, 0)
	}
	n, e := m.Z.Natural.appendDisplay(0, "(/ ")
	if e != nil {
		return 0, e
	}
	n, e = m.Z.prepareSMT2(&a.Num, decimal, n)
	if e != nil {
		return 0, e
	}
	n, e = m.Z.Natural.appendDisplay(n, " ")
	if e != nil {
		return 0, e
	}
	n, e = m.Z.prepareSMT2(&a.Den, decimal, n)
	if e != nil {
		return 0, e
	}
	return m.Z.Natural.appendDisplay(n, ")")
}
func (m *RationalManager) FormatSMT2Into(a *Rational, decimal bool, dst []byte) (int, error) {
	n, e := m.prepareSMT2(a, decimal)
	if e != nil {
		return 0, e
	}
	return m.Z.Natural.publishText(dst, n)
}
func (m *RationalManager) SMT2String(a *Rational, decimal bool) (string, error) {
	n, e := m.prepareSMT2(a, decimal)
	if e != nil {
		return "", e
	}
	return m.Z.Natural.finalString(n)
}

// prepareDecimal retains source Copy, sign/Neg, Div/display/Rem, then each
// Mul/Div/display/Rem action. No rounding or zero padding is introduced.
func (m *RationalManager) prepareDecimal(a *Rational, precision uint32, truncate bool) (length uint32, resultErr error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if e := m.Z.beginRawDisplay(3, [3]uint8{0, 1, 2}); e != nil {
		return 0, e
	}
	defer m.Z.finishRawDisplay(&resultErr, &length)
	numerator, denominator, digit := &m.Z.RawDisplay.Roots[0], &m.Z.RawDisplay.Roots[1], &m.Z.RawDisplay.Roots[2]
	ten := NewInt(10)
	if e := m.Z.Copy(numerator, &a.Num); e != nil {
		return 0, e
	}
	if e := m.Z.Copy(denominator, &a.Den); e != nil {
		return 0, e
	}
	var offset uint32
	if a.Num.Val < 0 {
		var e error
		offset, e = m.Z.Natural.appendDisplay(offset, "-")
		if e != nil {
			return 0, e
		}
		if e = m.Z.Neg(numerator); e != nil {
			return 0, e
		}
	}
	if e := m.Z.Div(numerator, denominator, digit); e != nil {
		return 0, e
	}
	offset, e := m.Z.prepareInteger(digit, offset)
	if e != nil {
		return 0, e
	}
	if e = m.Z.Rem(numerator, denominator, numerator); e != nil {
		return 0, e
	}
	if numerator.Val == 0 {
		return offset, nil
	}
	offset, e = m.Z.Natural.appendDisplay(offset, ".")
	if e != nil {
		return 0, e
	}
	for i := uint32(0); i < precision; i++ {
		if e = m.Z.Mul(numerator, &ten, numerator); e != nil {
			return 0, e
		}
		if e = m.Z.Div(numerator, denominator, digit); e != nil {
			return 0, e
		}
		offset, e = m.Z.prepareInteger(digit, offset)
		if e != nil {
			return 0, e
		}
		if e = m.Z.Rem(numerator, denominator, numerator); e != nil {
			return 0, e
		}
		if numerator.Val == 0 {
			return offset, nil
		}
	}
	if !truncate {
		return m.Z.Natural.appendDisplay(offset, "?")
	}
	return offset, nil
}
func (m *RationalManager) FormatDecimalInto(a *Rational, precision uint32, truncate bool, dst []byte) (int, error) {
	n, e := m.prepareDecimal(a, precision, truncate)
	if e != nil {
		return 0, e
	}
	return m.Z.Natural.publishText(dst, n)
}
func (m *RationalManager) DecimalString(a *Rational, precision uint32, truncate bool) (string, error) {
	n, e := m.prepareDecimal(a, precision, truncate)
	if e != nil {
		return "", e
	}
	return m.Z.Natural.finalString(n)
}

// ToRationalString and FormatPrettyInto are the source thin forwarding variants,
// retaining the ordinary integer-or-fraction representation and host boundary.
func (m *RationalManager) ToRationalString(a *Rational) (string, error) { return m.String(a) }
func (m *RationalManager) FormatPrettyInto(a *Rational, dst []byte) (int, error) {
	return m.FormatInto(a, dst)
}
