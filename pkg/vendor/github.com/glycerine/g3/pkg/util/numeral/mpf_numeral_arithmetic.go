package numeral

// Source: src/util/f2n.h:96-112, instantiated as f2n<mpf_manager>.

// Inv follows the source local-one, divide, delete, final-check sequence.
func (m *ArbitraryFloatNumeralManager) Inv(x *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.beginRawLocal(); e != nil {
		return e
	}
	if e := m.SetInt32(&m.rawLocal, 1); e != nil {
		return e
	}
	if e := m.Div(&m.rawLocal, x, x); e != nil {
		return e
	}
	if e := m.finishRawLocalSource(); e != nil {
		return e
	}
	return m.check(x)
}

// InvFrom is the source two-numeral overload used by interval division.
func (m *ArbitraryFloatNumeralManager) InvFrom(in, out *ArbitraryFloat) error {
	if e := m.Copy(out, in); e != nil {
		return e
	}
	return m.Inv(out)
}

func (m *ArbitraryFloatNumeralManager) Increment(x *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	return m.Add(x, &m.one, x)
}

func (m *ArbitraryFloatNumeralManager) Decrement(x *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	return m.Sub(x, &m.one, x)
}

// Power uses the source's unsigned mask walk, including its final square.
// A wrapped mask reports the source's nonterminating path after its reached
// operations and leaves the raw local for owning cleanup.
func (m *ArbitraryFloatNumeralManager) Power(a *ArbitraryFloat, exponent uint32, out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.beginRawLocal(); e != nil {
		return e
	}
	if e := m.Copy(&m.rawLocal, a); e != nil {
		return e
	}
	if e := m.SetInt32(out, 1); e != nil {
		return e
	}
	for mask := uint32(1); mask <= exponent; {
		if mask&exponent != 0 {
			if e := m.Mul(out, &m.rawLocal, out); e != nil {
				return e
			}
		}
		if e := m.Mul(&m.rawLocal, &m.rawLocal, &m.rawLocal); e != nil {
			return e
		}
		mask <<= 1
		if mask == 0 {
			return &PowerDomainError{Exponent: exponent, Reason: PowerMaskOverflow}
		}
	}
	if e := m.finishRawLocalSource(); e != nil {
		return e
	}
	return m.check(out)
}
