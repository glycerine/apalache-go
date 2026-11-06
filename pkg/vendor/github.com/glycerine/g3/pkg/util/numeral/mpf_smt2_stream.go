package numeral

// Source: src/util/mpf.cpp:1584-1588; src/util/mpf.h:143.

import (

	// DisplaySMT2Into converts through the original scoped rational and borrows
	// the caller stream. CleanupSMT2Stream joins actual failed children and locals.
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ArbitraryFloatManager) DisplaySMT2Into(out RationalSourceStream, x *ArbitraryFloat, decimal bool) (cause error) {
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if m.smt2Stream.active {
		return ErrNumericPending
	}
	if out == nil {
		return utext.ErrOwnedStringOwner
	}
	s := &m.smt2Stream
	*s = arbitraryFloatSMT2StreamScope{manager: m, q: NewRational(), qLive: true, active: true, executing: true}
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishSMT2StreamSource())
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
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
	before = m.rational.smt2Stream.invocation
	e = m.rational.DisplaySMT2Into(out, &s.q, decimal)
	if m.rational.smt2Stream.invocation != before {
		s.format = m.rational.smt2Stream.invocation
	}
	if e != nil {
		return e
	}
	s.formatOwned = true
	return nil
}

type arbitraryFloatSMT2StreamScope struct {
	manager                             *ArbitraryFloatManager
	q                                   Rational
	conversion, format                  uint64
	conversionOwned, formatOwned, qLive bool
	qCursor                             uint8
	integerBinding, naturalBinding      NumericCallerBinding
	active, executing, finishing        bool
	bodyCause, sourceCause              error
}

func (s *arbitraryFloatSMT2StreamScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.smt2Stream
}
func (s *arbitraryFloatSMT2StreamScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatSMT2StreamScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 32); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 32); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatSMT2StreamScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}

func (m *ArbitraryFloatManager) finishSMT2StreamSource() error {
	s := &m.smt2Stream
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
		if m.rational.smt2Stream.invocation != s.format {
			return ErrNumericOwner
		}
		if e := m.rational.smt2Stream.sourceCause; e != nil {
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

func (m *ArbitraryFloatManager) CleanupSMT2Stream() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.smt2Stream
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
		if m.rational.smt2Stream.invocation != s.format {
			return ErrNumericOwner
		}
		if e := m.rational.CleanupSMT2Stream(); e != nil {
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
	s.active = false
	return nil
}
