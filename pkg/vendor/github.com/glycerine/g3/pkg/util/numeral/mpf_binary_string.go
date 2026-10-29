package numeral

// Source: src/util/mpf.cpp:1635-1693; src/util/mpf.h:295.

import (

	// ToStringBinary follows the source text construction and repeated integer
	// division, constructing the returned string in fresh caller storage.
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ArbitraryFloatManager) ToStringBinary(x *ArbitraryFloat, upperExtra, lowerExtra uint32, out *utext.OwnedString) (cause error) {
	if e := m.valueGuard(x); e != nil {
		return e
	}
	s := &m.binaryString
	if s.active || s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	if out == nil || out.Self != nil {
		return utext.ErrOwnedStringOwner
	}
	*s = arbitraryFloatBinaryStringScope{manager: m, invocation: s.invocation + 1, out: out, active: true, executing: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishBinaryStringSource())
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
			s.out = nil
			s.outLive = false
			s.active = false
		}
	}()
	if e := out.Construct(m.budget, ""); e != nil {
		return e
	}
	s.outLive = true
	return s.run(x, upperExtra, lowerExtra)
}

func (s *arbitraryFloatBinaryStringScope) appendExpr(text string) error {
	return s.expr.AppendCString(text)
}
func (s *arbitraryFloatBinaryStringScope) beginExpr() error {
	s.expr = utext.OwnedString{}
	e := s.expr.Construct(s.manager.budget, "")
	s.exprLive = s.expr.Ready
	return e
}
func (s *arbitraryFloatBinaryStringScope) repeatExpr(index int, count uint32, ch byte) error {
	e := s.repeats[index].ConstructRepeated(s.manager.budget, uint64(count), ch)
	s.repeatLive[index] = e == nil
	if e != nil {
		return e
	}
	// For these exact count constructors the total combined length always
	// exceeds a heap rhs capacity, so the selected rvalue+rvalue overload
	// retains and appends to its left operand.
	return s.expr.Append(s.repeats[index].View())
}
func (s *arbitraryFloatBinaryStringScope) endExpr() {
	for i := 1; i >= 0; i-- {
		if s.repeatLive[i] {
			s.repeats[i].SourceDestroy()
			s.repeatLive[i] = false
		}
	}
	if s.exprLive {
		s.expr.SourceDestroy()
		s.exprLive = false
	}
}
func (s *arbitraryFloatBinaryStringScope) assignExpr() error {
	if e := s.out.MoveAssign(&s.expr); e != nil {
		return e
	}
	s.endExpr()
	return nil
}
func (s *arbitraryFloatBinaryStringScope) run(x *ArbitraryFloat, upper, lower uint32) error {
	m, z := s.manager, s.manager.integer
	nan, e := m.IsNaN(x)
	if e != nil {
		return e
	}
	if nan {
		if e := s.beginExpr(); e != nil {
			return e
		}
		if e := s.appendExpr("#b0 "); e != nil {
			return e
		}
		if e := s.appendExpr("#b"); e != nil {
			return e
		}
		if e := s.repeatExpr(0, x.EBits(), '1'); e != nil {
			return e
		}
		if e := s.appendExpr(" "); e != nil {
			return e
		}
		if e := s.appendExpr("#b"); e != nil {
			return e
		}
		if e := s.repeatExpr(1, x.SBits()-2, '0'); e != nil {
			return e
		}
		if e := s.appendExpr("1 "); e != nil {
			return e
		}
		if e := s.appendExpr("(NaN)"); e != nil {
			return e
		}
		return s.assignExpr()
	}
	inf, e := m.IsInf(x)
	if e != nil {
		return e
	}
	zero := false
	if !inf {
		zero, e = m.IsZero(x)
		if e != nil {
			return e
		}
	}
	sign, polarity := "0", "+"
	if x.Sign() {
		sign, polarity = "1", "-"
	}
	if inf || zero {
		if e := s.beginExpr(); e != nil {
			return e
		}
		if e := s.appendExpr("#b"); e != nil {
			return e
		}
		if e := s.appendExpr(sign); e != nil {
			return e
		}
		if e := s.appendExpr(" "); e != nil {
			return e
		}
		if e := s.appendExpr("#b"); e != nil {
			return e
		}
		exponentCh := byte('0')
		if inf {
			exponentCh = '1'
		}
		if e := s.repeatExpr(0, x.EBits(), exponentCh); e != nil {
			return e
		}
		if e := s.appendExpr(" "); e != nil {
			return e
		}
		if e := s.appendExpr("#b"); e != nil {
			return e
		}
		if e := s.repeatExpr(1, x.SBits()-1, '0'); e != nil {
			return e
		}
		if inf {
			if e := s.appendExpr("1 "); e != nil {
				return e
			}
		} else {
			if e := s.appendExpr(" "); e != nil {
				return e
			}
		}
		if e := s.appendExpr("("); e != nil {
			return e
		}
		if e := s.appendExpr(polarity); e != nil {
			return e
		}
		ending := "zero)"
		if inf {
			ending = "oo)"
		}
		if e := s.appendExpr(ending); e != nil {
			return e
		}
		return s.assignExpr()
	}
	assertionsEnabled, e := mpfSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if assertionsEnabled {
		negative, e := z.IsNeg(&x.significand)
		if e != nil {
			return e
		}
		if e := mpfSourceAssertion(!negative, 1654, "m_mpz_manager.is_nonneg(sig(x))"); e != nil {
			return e
		}
	}
	if e := s.beginExpr(); e != nil {
		return e
	}
	if e := s.appendExpr("#b"); e != nil {
		return e
	}
	if e := s.appendExpr(sign); e != nil {
		return e
	}
	if e := s.appendExpr(" "); e != nil {
		return e
	}
	if e := s.assignExpr(); e != nil {
		return e
	}
	s.tmp = NewInt(0)
	s.tmpLive = true
	denormal, e := m.IsDenormal(x)
	if e != nil {
		return e
	}
	exponent := x.exponent
	if denormal {
		exponent, e = m.MkMinExp(x.EBits())
		if e != nil {
			return e
		}
	}
	biased, e := m.BiasExp(x.EBits(), exponent)
	if e != nil {
		return e
	}
	c := integerAllocationInvocationContext{Caller: &s.hooks}
	if e := z.setInt64WithAllocationContext(&s.tmp, biased, c); e != nil {
		return e
	}
	if e := s.bits.Construct(m.budget, ""); e != nil {
		return e
	}
	s.bitsLive = true
	two := NewInt(2)
	for i := uint32(0); i < x.EBits(); i++ {
		if e := s.appendBit(); e != nil {
			return e
		}
		if e := z.divWithAllocationContext(&s.tmp, &two, &s.tmp, c); e != nil {
			return e
		}
	}
	if e := s.bits.Reverse(); e != nil {
		return e
	}
	if e := s.appendBits(true); e != nil {
		return e
	}
	if e := s.bits.Assign(""); e != nil {
		return e
	}
	if e := z.copyFromOwnerWithAllocationContext(&s.tmp, z, &x.significand, nil, c); e != nil {
		return e
	}
	numBits := upper + x.SBits() + lower
	for i := uint32(0); i < numBits || s.tmp.Val != 0; i++ {
		if e := s.appendBit(); e != nil {
			return e
		}
		if e := z.divWithAllocationContext(&s.tmp, &two, &s.tmp, c); e != nil {
			return e
		}
		if i == lower-1 {
			if e := s.bits.AppendCString(","); e != nil {
				return e
			}
		}
		if i == x.SBits()+lower-2 {
			if e := s.bits.AppendCString("."); e != nil {
				return e
			}
			if i == numBits-1 {
				if e := s.bits.AppendCString(" "); e != nil {
					return e
				}
			}
		}
	}
	if e := s.bits.Reverse(); e != nil {
		return e
	}
	return s.appendBits(false)
}
func (s *arbitraryFloatBinaryStringScope) appendBit() error {
	odd, e := s.manager.integer.IsOdd(&s.tmp)
	if e != nil {
		return e
	}
	bit := "0"
	if odd {
		bit = "1"
	}
	return s.bits.AppendCString(bit)
}
func (s *arbitraryFloatBinaryStringScope) appendBits(space bool) error {
	s.expr = utext.OwnedString{}
	e := s.expr.ConstructCStringConcat(s.manager.budget, "#b", s.bits.View())
	s.exprLive = s.expr.Ready
	if e != nil {
		return e
	}
	if space {
		if e := s.appendExpr(" "); e != nil {
			return e
		}
	}
	if e := s.out.Append(s.expr.View()); e != nil {
		return e
	}
	s.endExpr()
	return nil
}

type arbitraryFloatBinaryStringScope struct {
	manager                              *ArbitraryFloatManager
	invocation                           uint64
	out                                  *utext.OwnedString
	tmp                                  Int
	bits, expr                           utext.OwnedString
	repeats                              [2]utext.OwnedString
	repeatLive                           [2]bool
	tmpLive, bitsLive, exprLive, outLive bool
	hooks                                NumericInvocationHooks
	numeric                              NumericAllocationRoots
	division                             integerRawInvocationReceipt

	integerBinding, naturalBinding NumericCallerBinding
	active, executing, finishing   bool
	bodyCause, sourceCause         error
}

func (s *arbitraryFloatBinaryStringScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.binaryString && s.invocation != 0
}
func (s *arbitraryFloatBinaryStringScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatBinaryStringScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && (s.manager.memory == nil || !s.manager.memory.FailureTransportActive()))
}
func (s *arbitraryFloatBinaryStringScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}
func (s *arbitraryFloatBinaryStringScope) NumericRecord(z *IntegerManager, event string, r NumericInvocationReceipt) {
	if s.NumericBound(true) && z == s.manager.integer && event == "division" && r.Raw != nil {
		s.division = *r.Raw
	}
}

func (s *arbitraryFloatBinaryStringScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 35); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 35); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatBinaryStringScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *ArbitraryFloatManager) finishBinaryStringSource() error {
	s := &m.binaryString
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	for _, e := range []error{m.integer.RawInvocationSourceCause(&s.division)} {
		if e != nil {
			s.sourceCause = e
			return e
		}
	}
	s.endExpr()
	if s.bitsLive {
		s.bits.SourceDestroy()
		s.bitsLive = false
	}
	if s.tmpLive {
		if _, e := SourceNumeralDel(m.integer, &s.tmp); e != nil {
			s.sourceCause = e
			return e
		}
		s.tmpLive = false
	}
	if s.bodyCause != nil && s.outLive {
		s.out.SourceDestroy()
		s.outLive = false
	}
	return nil
}

// CleanupBinaryString joins only this invocation's actual failed prefixes and
// children. It does not replay a stopped source destructor or numerical body.
func (m *ArbitraryFloatManager) CleanupBinaryString() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.binaryString
	if s.executing || s.finishing || m.valueExecuting || m.memory != nil && m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if !s.valid() {
		return ErrNumericOwner
	}
	if e := s.bind(); e != nil {
		return e
	}
	s.finishing = true
	defer func() { s.finishing = false; cause = JoinNumericErrors(cause, s.unbind()) }()
	z := m.integer

	if e := z.CleanupRawInvocation(&s.division, true); e != nil {
		return e
	}
	s.expr.OwnAbort()
	for i := 1; i >= 0; i-- {
		s.repeats[i].OwnAbort()
		s.repeatLive[i] = false
	}
	s.bits.OwnAbort()
	s.bitsLive = false
	s.exprLive = false
	if s.tmpLive {
		if e := z.CleanupRawNumeral(&s.tmp); e != nil {
			return e
		}
		s.tmpLive = false
	}
	if s.out != nil {
		s.out.OwnAbort()
		s.outLive = false
	}

	if e := s.numeric.Cleanup(z); e != nil {
		return e
	}
	s.out = nil
	s.active = false
	return nil
}
func (s *arbitraryFloatBinaryStringScope) storageBytes() uint64 {
	var n uint64
	for _, str := range [...]*utext.OwnedString{&s.bits, &s.expr, &s.repeats[0], &s.repeats[1], s.out} {
		if str != nil {
			n += str.StorageBytes()
		}
	}
	return n
}
