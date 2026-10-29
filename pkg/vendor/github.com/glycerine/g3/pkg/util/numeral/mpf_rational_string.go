package numeral

// Source: src/util/mpf.cpp:1572-1576; src/util/mpf.h:141.

import (

	// ToRationalString converts through the original scoped rational and constructs
	// a caller-owned result in fresh storage. CleanupRationalString joins failures.
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ArbitraryFloatManager) ToRationalString(x *ArbitraryFloat, out *utext.OwnedString) (cause error) {
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if m.rationalString.active {
		return ErrNumericPending
	}
	if out == nil || out.Self != nil {
		return utext.ErrOwnedStringOwner
	}
	s := &m.rationalString
	*s = arbitraryFloatRationalStringScope{manager: m, q: NewRational(), qLive: true, out: out, active: true, executing: true}
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishRationalStringSource())
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
			s.out = nil
			s.outputOwned = false
			s.active = false
		}
	}()
	before := m.rationalConversionInvocation
	e := m.ToRational(x, m.rational, &s.q)
	if m.rationalConversionInvocation != before {
		s.conversion = m.rationalConversionInvocation
	}
	if e != nil {
		return e
	}
	// Join only the actually completed child's adapter receipts before later
	// integer formatting can reuse its primitive frames.
	if s.conversion != 0 {
		if e := m.CleanupRationalConversion(); e != nil {
			return e
		}
		s.conversionOwned = true
	}
	before = m.rational.stringScope.invocation
	e = m.rational.ToOwnedString(&s.q, out)
	if m.rational.stringScope.invocation != before {
		s.format = m.rational.stringScope.invocation
	}
	if e != nil {
		return e
	}
	s.formatOwned = true
	s.outputOwned = true
	return nil
}

type arbitraryFloatRationalStringScope struct {
	manager                                          *ArbitraryFloatManager
	q                                                Rational
	out                                              *utext.OwnedString
	conversion, format                               uint64
	conversionOwned, formatOwned, outputOwned, qLive bool
	qCursor                                          uint8
	integerBinding, naturalBinding                   NumericCallerBinding
	active, executing, finishing                     bool
	bodyCause, sourceCause                           error
}

func (s *arbitraryFloatRationalStringScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.rationalString
}
func (s *arbitraryFloatRationalStringScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatRationalStringScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 29); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 29); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatRationalStringScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}

func (m *ArbitraryFloatManager) finishRationalStringSource() error {
	s := &m.rationalString
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
	if s.format != 0 && !s.formatOwned {
		if m.rational.stringScope.invocation != s.format {
			return ErrNumericOwner
		}
		if e := m.rational.stringScope.sourceCause; e != nil {
			s.sourceCause = e
			return e
		}
	}
	if s.conversion != 0 && !s.conversionOwned {
		if m.rationalConversion.invocation != s.conversion {
			return ErrNumericOwner
		}
		if e := m.rationalConversion.sourceCause; e != nil {
			s.sourceCause = e
			return e
		}
	}
	for s.qLive && s.qCursor < 2 {
		v := &s.q.Num
		if s.qCursor == 1 {
			v = &s.q.Den
		}
		if _, e := SourceNumeralDel(m.integer, v); e != nil {
			s.sourceCause = e
			return e
		}
		s.qCursor++
	}
	s.qLive = false
	return nil
}

func (m *ArbitraryFloatManager) CleanupRationalString() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.rationalString
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
	if s.format != 0 && !s.formatOwned {
		if m.rational.stringScope.invocation != s.format {
			return ErrNumericOwner
		}
		if e := m.rational.CleanupOwnedString(); e != nil {
			return e
		}
		s.formatOwned = true
	}
	if s.conversion != 0 && !s.conversionOwned {
		if m.rationalConversion.invocation != s.conversion {
			return ErrNumericOwner
		}
		if e := m.CleanupRationalConversion(); e != nil {
			return e
		}
		s.conversionOwned = true
	}
	for s.qLive && s.qCursor < 2 {
		v := &s.q.Num
		if s.qCursor == 1 {
			v = &s.q.Den
		}
		if e := m.integer.CleanupRawNumeral(v); e != nil {
			return e
		}
		s.qCursor++
	}
	s.qLive = false
	if s.outputOwned && s.out != nil {
		s.out.OwnAbort()
		s.outputOwned = false
	}
	s.out = nil
	s.active = false
	return nil
}
func (s *arbitraryFloatRationalStringScope) storageBytes() uint64 {
	if s.outputOwned && s.out != nil {
		return s.out.StorageBytes()
	}
	return 0
}
