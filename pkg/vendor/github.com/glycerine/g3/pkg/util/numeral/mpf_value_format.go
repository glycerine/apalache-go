package numeral

// Source: src/util/mpf.cpp:1519-1569.

// ToStringInto writes the source result into its actual owned text context.
// The returned c_str view borrows that result until the caller's result DT.
func (m *ArbitraryFloatManager) ToStringInto(a *ArbitraryFloat, text ArbitraryFloatFormatTarget) (view ArbitraryFloatFormattedCString, err error) {
	return m.toStringIntoWithReceipt(a, text, nil, nil)
}

// Capture the exact invocation at successful begin, before text body callbacks.
// Refused entry issues no receipt, even if a callback begins another frame.
func (m *ArbitraryFloatManager) toStringIntoWithReceipt(a *ArbitraryFloat, text ArbitraryFloatFormatTarget, frame **ArbitraryFloatFormatInvocation, invocation *uint64) (view ArbitraryFloatFormattedCString, err error) {
	if e := m.valueGuard(a); e != nil {
		return view, e
	}
	if text == nil {
		return view, ErrNumericOwner
	}
	s := text.NumericInvocation()
	if e := m.beginFormatInvocation(s, text); e != nil {
		return view, e
	}
	if frame != nil && invocation != nil {
		*frame = s
		*invocation = s.invocation
	}
	defer func() {
		s.executing = false
		s.bodyCause = err
		source := m.finishFormatSource(s)
		err = JoinNumericErrors(err, source)
		if err == nil {
			s.returned = true
			view, err = text.CString()
			if err != nil {
				s.bodyCause = err
				s.returned = false
			}
		}
	}()
	nan, e := m.IsNaN(a)
	if e != nil {
		return view, e
	}
	if nan {
		return view, text.AssignLiteral("NaN")
	}
	inf, e := m.IsInf(a)
	if e != nil {
		return view, e
	}
	if inf {
		if a.Sign() {
			return view, text.AssignLiteral("-oo")
		}
		return view, text.AssignLiteral("+oo")
	}
	zero, e := m.IsZero(a)
	if e != nil {
		return view, e
	}
	if zero {
		if a.Sign() {
			return view, text.AssignLiteral("-zero")
		}
		return view, text.AssignLiteral("+zero")
	}
	prefix := ""
	if a.Sign() {
		prefix = "-"
	}
	if e := text.AssignLiteral(prefix); e != nil {
		return view, e
	}
	// Scoped NUM constructors are reached only on the finite branch.
	s.Num = Int{}
	s.Constructed = 1
	s.Denom = Int{}
	s.Constructed = 2
	context := s.allocationContext()
	z := m.integer
	if e := z.setInt64WithAllocationContext(&s.Num, 0, context); e != nil {
		return view, e
	}
	if e := z.setInt64WithAllocationContext(&s.Denom, 1, context); e != nil {
		return view, e
	}
	denormal, e := m.IsDenormal(a)
	if e != nil {
		return view, e
	}
	exponent := a.exponent
	if denormal {
		exponent, e = m.minExponent(a.EBits())
		if e != nil {
			return view, e
		}
	} else {
		if e := z.setInt64WithAllocationContext(&s.Num, 1, context); e != nil {
			return view, e
		}
		if e := z.shiftWithAllocationContext(&s.Num, a.SBits()-1, &s.Num, true, context); e != nil {
			return view, e
		}
	}
	if e := z.addSubWithAllocationContext(&s.Num, &a.significand, &s.Num, false, context); e != nil {
		return view, e
	}
	if e := z.shiftWithAllocationContext(&s.Denom, a.SBits()-1, &s.Denom, true, context); e != nil {
		return view, e
	}
	s.Q = NewRational()
	s.Constructed = 3
	if e := m.rational.setIntegerWithAllocationContext(&s.Q, &s.Num, context); e != nil {
		return view, e
	}
	if e := m.rational.divIntWithAllocationContext(&s.Q, &s.Denom, &s.Q, context); e != nil {
		return view, e
	}
	s.stream, e = text.NewArbitraryFloatFormatStream()
	if e != nil {
		return view, e
	}
	if e := m.displayFormatDecimal(s, s.stream, &s.Q, a.SBits()); e != nil {
		return view, e
	}
	integer, e := m.rational.IsInt(&s.Q)
	if e != nil {
		return view, e
	}
	if integer {
		if e := s.stream.AppendCString(".0"); e != nil {
			return view, e
		}
	}
	if e := s.stream.AppendCString(" "); e != nil {
		return view, e
	}
	if e := s.stream.AppendInt64(exponent); e != nil {
		return view, e
	}
	return view, text.AppendStreamString(s.stream)
}
func (m *ArbitraryFloatManager) finishFormatSource(s *ArbitraryFloatFormatInvocation) error {
	if !m.MatchesFormatFrame(s) {
		return ErrNumericOwner
	}
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if e := m.formatChildSource(s); e != nil {
		s.sourceCause = e
		return e
	}
	for s.sourceCursor < 5 {
		var e error
		switch s.sourceCursor {
		case 0:
			if s.stream != nil {
				e = s.stream.SourceFinalize()
			}
		case 1:
			if s.Constructed >= 3 {
				_, e = SourceNumeralDel(m.integer, &s.Q.Num)
			}
		case 2:
			if s.Constructed >= 3 {
				_, e = SourceNumeralDel(m.integer, &s.Q.Den)
			}
		case 3:
			if s.Constructed >= 2 {
				_, e = SourceNumeralDel(m.integer, &s.Denom)
			}
		case 4:
			if s.Constructed >= 1 {
				_, e = SourceNumeralDel(m.integer, &s.Num)
			}
		}
		if e != nil {
			s.sourceCause = e
			return e
		}
		s.sourceCursor++
	}
	return nil
}
