package numeral

// Source: src/util/mpf.cpp:373-391; src/util/mpf.h:92-96.

// Abs clears only the source sign bit, including for NaN.
func (m *ArbitraryFloatManager) Abs(out *ArbitraryFloat) error {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	out.assignSign(false)
	return nil
}

// AbsCopy retains SET's actual allocation receipt before applying the sign bit.
func (m *ArbitraryFloatManager) AbsCopy(out, source *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) error {
	if e := m.CopyWithReceipt(out, source, receipt); e != nil {
		return e
	}
	return m.Abs(out)
}

// Neg classifies first. The source leaves a NaN's sign unchanged.
func (m *ArbitraryFloatManager) Neg(out *ArbitraryFloat) error {
	nan, e := m.IsNaN(out)
	if e != nil {
		return e
	}
	if !nan {
		out.assignSign(!out.Sign())
	}
	return nil
}

func (m *ArbitraryFloatManager) NegCopy(out, source *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) error {
	if e := m.CopyWithReceipt(out, source, receipt); e != nil {
		return e
	}
	return m.Neg(out)
}
