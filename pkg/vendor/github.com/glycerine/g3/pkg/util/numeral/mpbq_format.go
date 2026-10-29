package numeral

// Source: MPBQ display methods.

func (m *BinaryRationalManager) appendUnsigned(offset, n uint32) (uint32, error) {
	end, e := checkedTextEnd(offset, 10)
	if e != nil {
		return 0, e
	}
	w := m.Z.Natural
	if e = w.reserveText(end); e != nil {
		return 0, e
	}
	i := offset
	for {
		w.textWrite(i, '0'+byte(n%10))
		i++
		n /= 10
		if n == 0 {
			break
		}
	}
	w.reverseText(offset, i)
	return i, nil
}
func (m *BinaryRationalManager) prepareBinary(a *BinaryRational, pretty bool) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	n, e := m.Z.prepareInteger(&a.Num, 0)
	if e != nil {
		return 0, e
	}
	if a.KField == 0 {
		return n, nil
	}
	w := m.Z.Natural
	n, e = w.appendDisplay(n, "/2")
	if e != nil || a.KField == 1 {
		return n, e
	}
	prefix := "^"
	if pretty {
		prefix = "<sup>"
	}
	n, e = w.appendDisplay(n, prefix)
	if e != nil {
		return 0, e
	}
	n, e = m.appendUnsigned(n, a.KField)
	if e != nil {
		return 0, e
	}
	if pretty {
		return w.appendDisplay(n, "</sup>")
	}
	return n, nil
}
func (m *BinaryRationalManager) String(a *BinaryRational) (string, error) {
	n, e := m.prepareBinary(a, false)
	if e != nil {
		return "", e
	}
	return m.Z.Natural.finalString(n)
}
func (m *BinaryRationalManager) FormatInto(a *BinaryRational, dst []byte) (int, error) {
	n, e := m.prepareBinary(a, false)
	if e != nil {
		return 0, e
	}
	return m.Z.Natural.publishText(dst, n)
}
func (m *BinaryRationalManager) PrettyString(a *BinaryRational) (string, error) {
	n, e := m.prepareBinary(a, true)
	if e != nil {
		return "", e
	}
	return m.Z.Natural.finalString(n)
}
func (m *BinaryRationalManager) FormatPrettyInto(a *BinaryRational, dst []byte) (int, error) {
	n, e := m.prepareBinary(a, true)
	if e != nil {
		return 0, e
	}
	return m.Z.Natural.publishText(dst, n)
}
func (m *BinaryRationalManager) prepareBinarySMT2(a *BinaryRational, decimal bool) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.KField == 0 {
		return m.Z.prepareSMT2(&a.Num, decimal, 0)
	}
	w := m.Z.Natural
	n, e := w.appendDisplay(0, "(/ ")
	if e != nil {
		return 0, e
	}
	n, e = m.Z.prepareSMT2(&a.Num, decimal, n)
	if e != nil {
		return 0, e
	}
	// The source inserts a separator and then an exponent term whose own
	// opening text starts with a space.
	n, e = w.appendDisplay(n, "  (^ 2")
	if e != nil {
		return 0, e
	}
	if decimal {
		n, e = w.appendDisplay(n, ".0")
		if e != nil {
			return 0, e
		}
	}
	n, e = w.appendDisplay(n, " ")
	if e != nil {
		return 0, e
	}
	n, e = m.appendUnsigned(n, a.KField)
	if e != nil {
		return 0, e
	}
	if decimal {
		n, e = w.appendDisplay(n, ".0")
		if e != nil {
			return 0, e
		}
	}
	return w.appendDisplay(n, "))")
}
func (m *BinaryRationalManager) SMT2String(a *BinaryRational, decimal bool) (string, error) {
	n, e := m.prepareBinarySMT2(a, decimal)
	if e != nil {
		return "", e
	}
	return m.Z.Natural.finalString(n)
}
func (m *BinaryRationalManager) FormatSMT2Into(a *BinaryRational, decimal bool, dst []byte) (int, error) {
	n, e := m.prepareBinarySMT2(a, decimal)
	if e != nil {
		return 0, e
	}
	return m.Z.Natural.publishText(dst, n)
}
func (m *BinaryRationalManager) prepareBinaryDecimal(a *BinaryRational, prec uint32) (result uint32, err error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.KField == 0 {
		return m.Z.prepareInteger(&a.Num, 0)
	}
	if e := m.beginBinaryDisplay(); e != nil {
		return 0, e
	}
	defer m.finishBinaryDisplay(&result, &err)
	two, ten := NewInt(2), NewInt(10)
	n1, v1, twok := &m.Display.Values[0], &m.Display.Values[1], &m.Display.Values[2]
	w := m.Z.Natural
	offset := uint32(0)
	var e error
	if a.Num.Val < 0 {
		offset, e = w.appendDisplay(0, "-")
		if e != nil {
			return 0, e
		}
	}
	if e = m.Z.Copy(v1, &a.Num); e != nil {
		return 0, e
	}
	if e = m.Z.Abs(v1); e != nil {
		return 0, e
	}
	if e = m.Z.Power(&two, a.KField, twok); e != nil {
		return 0, e
	}
	if e = m.Z.Rem(v1, twok, n1); e != nil {
		return 0, e
	}
	if e = m.Z.Div(v1, twok, v1); e != nil {
		return 0, e
	}
	if e = m.debugAssert(n1.Val != 0); e != nil {
		return 0, e
	}
	offset, e = m.Z.prepareInteger(v1, offset)
	if e != nil {
		return 0, e
	}
	offset, e = w.appendDisplay(offset, ".")
	if e != nil {
		return 0, e
	}
	for i := uint32(0); i < prec; i++ {
		if e = m.Z.Mul(n1, &ten, n1); e != nil {
			return 0, e
		}
		if e = m.Z.Div(n1, twok, v1); e != nil {
			return 0, e
		}
		if e = m.Z.Rem(n1, twok, n1); e != nil {
			return 0, e
		}
		offset, e = m.Z.prepareInteger(v1, offset)
		if e != nil {
			return 0, e
		}
		if n1.Val == 0 {
			return offset, nil
		}
	}
	return w.appendDisplay(offset, "?")
}
func (m *BinaryRationalManager) DecimalString(a *BinaryRational, prec uint32) (string, error) {
	n, e := m.prepareBinaryDecimal(a, prec)
	if e != nil {
		return "", e
	}
	return m.Z.Natural.finalString(n)
}
func (m *BinaryRationalManager) DecimalStringDefault(a *BinaryRational) (string, error) {
	return m.DecimalString(a, 8)
}
func (m *BinaryRationalManager) FormatDecimalInto(a *BinaryRational, prec uint32, dst []byte) (int, error) {
	n, e := m.prepareBinaryDecimal(a, prec)
	if e != nil {
		return 0, e
	}
	return m.Z.Natural.publishText(dst, n)
}
