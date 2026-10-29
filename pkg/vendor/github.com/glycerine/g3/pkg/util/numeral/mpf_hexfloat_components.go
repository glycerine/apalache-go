package numeral

// Source: src/util/mpf.cpp:1608-1619; src/util/mpf.h:294.

import (

	// ToStringHexfloatComponentsInto restores scale and strips the hidden bit,
	// then borrows the caller's ordinary MPF text context for the source return.
	// Despite its source name this overload delegates to to_string, not hex output.
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ArbitraryFloatManager) ToStringHexfloatComponentsInto(sign bool, exponent int64, sig *Int, ebits, sbits, rbits uint32, text ArbitraryFloatFormatTarget) (view ArbitraryFloatFormattedCString, cause error) {
	if e := m.valueOwnerGuard(); e != nil {
		return view, e
	}
	if e := m.integer.check(sig); e != nil {
		return view, e
	}
	s := &m.hexfloatComponents
	if s.active || s.invocation == ^uint64(0) {
		return view, ErrNumericPending
	}
	if text == nil {
		return view, utext.ErrOwnedStringOwner
	}
	*s = arbitraryFloatHexfloatComponentsScope{manager: m, invocation: s.invocation + 1, qLive: true, sigLive: true, active: true, executing: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return view, e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishHexfloatComponentsSource())
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
			s.active = false
		} else {
			view = ArbitraryFloatFormattedCString{}
		}
	}()
	z, c := m.integer, integerAllocationInvocationContext{Caller: &s.hooks}
	if e := z.copyFromOwnerWithAllocationContext(&s.sig, z, sig, nil, c); e != nil {
		return view, e
	}
	if rbits != 0 {
		divisor, e := s.power(rbits)
		if e != nil {
			return view, e
		}
		if e := z.divWithAllocationContext(&s.sig, &divisor, &s.sig, c); e != nil {
			return view, e
		}
	}
	hidden, e := s.power(sbits - 1)
	if e != nil {
		return view, e
	}
	ge, e := z.GreaterEqual(&s.sig, &hidden)
	if e != nil {
		return view, e
	}
	if ge {
		// Preserve the second reached source powers2 lookup rather than caching
		// its first returned numeral as authority for a later argument read.
		hidden, e = s.power(sbits - 1)
		if e != nil {
			return view, e
		}
		if e := z.addSubWithAllocationContext(&s.sig, &hidden, &s.sig, true, c); e != nil {
			return view, e
		}
	} else {
		min, e := m.MkMinExp(ebits)
		if e != nil {
			return view, e
		}
		if exponent == min {
			exponent, e = m.MkBotExp(ebits)
			if e != nil {
				return view, e
			}
		}
	}
	// Literal mpz-significand SET: publish format/sign, COPY, then exponent.
	s.q.format = arbitraryFloatFormat(ebits, sbits, sign)
	if e := z.copyFromOwnerWithAllocationContext(&s.q.significand, z, &s.sig, nil, c); e != nil {
		return view, e
	}
	s.q.exponent = exponent
	view, e = m.toStringIntoWithReceipt(&s.q, text, &s.format, &s.formatInvocation)
	if e != nil {
		return view, e
	}
	// A completed child's NUM receipts must be joined before the older scoped
	// values are destroyed. The returned text backing remains caller-owned.
	if s.format != nil {
		if e := m.CleanupFormatInvocation(s.format, true); e != nil {
			return view, e
		}
		s.formatOwned = true
	}
	return view, nil
}

type arbitraryFloatHexfloatComponentsScope struct {
	manager          *ArbitraryFloatManager
	invocation       uint64
	q                ArbitraryFloat
	sig              Int
	qLive, sigLive   bool
	format           *ArbitraryFloatFormatInvocation
	formatInvocation uint64
	formatOwned      bool
	hooks            NumericInvocationHooks
	numeric          NumericAllocationRoots
	division         integerRawInvocationReceipt

	integerBinding, naturalBinding NumericCallerBinding
	active, executing, finishing   bool
	bodyCause, sourceCause         error
}

func (s *arbitraryFloatHexfloatComponentsScope) power(bits uint32) (Int, error) {
	w, e := s.manager.powerTwoWrapper(bits, false)
	if e != nil {
		return Int{}, e
	}
	return w.Capture()
}
func (s *arbitraryFloatHexfloatComponentsScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.hexfloatComponents && s.invocation != 0
}
func (s *arbitraryFloatHexfloatComponentsScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatHexfloatComponentsScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && (s.manager.memory == nil || !s.manager.memory.FailureTransportActive()))
}
func (s *arbitraryFloatHexfloatComponentsScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}
func (s *arbitraryFloatHexfloatComponentsScope) NumericRecord(z *IntegerManager, event string, r NumericInvocationReceipt) {
	if s.NumericBound(true) && z == s.manager.integer && event == "division" && r.Raw != nil {
		s.division = *r.Raw
	}
}

func (s *arbitraryFloatHexfloatComponentsScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 36); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 36); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatHexfloatComponentsScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (s *arbitraryFloatHexfloatComponentsScope) formatMatches() bool {
	f := s.format
	return f != nil && f.self == f && f.manager == s.manager && f.invocation == s.formatInvocation && (f.owned || s.manager.MatchesFormatFrame(f))
}
func (m *ArbitraryFloatManager) finishHexfloatComponentsSource() error {
	s := &m.hexfloatComponents
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
	if s.format != nil && !s.formatOwned {
		if !s.formatMatches() {
			return ErrNumericOwner
		}
		if e := s.format.sourceCause; e != nil {
			s.sourceCause = e
			return e
		}
	}
	for _, e := range []error{m.integer.RawInvocationSourceCause(&s.division)} {
		if e != nil {
			s.sourceCause = e
			return e
		}
	}
	if s.sigLive {
		if _, e := SourceNumeralDel(m.integer, &s.sig); e != nil {
			s.sourceCause = e
			return e
		}
		s.sigLive = false
	}
	if s.qLive {
		if _, e := SourceNumeralDel(m.integer, &s.q.significand); e != nil {
			s.sourceCause = e
			return e
		}
		s.qLive = false
	}
	return nil
}

// CleanupHexfloatComponents joins only actually reached child receipts and
// scoped values. It leaves borrowed input and caller text storage untouched.
func (m *ArbitraryFloatManager) CleanupHexfloatComponents() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.hexfloatComponents
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
	if s.format != nil && !s.formatOwned {
		if !s.formatMatches() {
			return ErrNumericOwner
		}
		if !s.format.owned {
			if e := m.CleanupFormatInvocation(s.format, true); e != nil {
				return e
			}
		}
		s.formatOwned = true
	}
	z := m.integer

	if e := z.CleanupRawInvocation(&s.division, true); e != nil {
		return e
	}
	if s.sigLive {
		if e := z.CleanupRawNumeral(&s.sig); e != nil {
			return e
		}
		s.sigLive = false
	}
	if s.qLive {
		if e := z.CleanupRawNumeral(&s.q.significand); e != nil {
			return e
		}
		s.qLive = false
	}

	if e := s.numeric.Cleanup(z); e != nil {
		return e
	}
	s.active = false
	return nil
}
