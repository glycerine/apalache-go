package numeral

// Source: src/util/mpf.cpp:1578-1582; src/util/mpf.h:142.

import (

	// DisplayDecimalInto converts through the original scoped rational and borrows
	// the caller stream. CleanupDecimalStream joins actual failed children and locals.
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ArbitraryFloatManager) DisplayDecimalInto(out RationalSourceStream, x *ArbitraryFloat, precision uint32) (cause error) {
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if m.decimalStream.active {
		return ErrNumericPending
	}
	if out == nil {
		return utext.ErrOwnedStringOwner
	}
	s := &m.decimalStream
	*s = arbitraryFloatDecimalStreamScope{manager: m, q: NewRational(), qLive: true, active: true, executing: true}
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishDecimalStreamSource())
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
	before = m.rational.decimalStream.invocation
	e = m.rational.DisplayDecimalInto(out, &s.q, precision, false)
	if m.rational.decimalStream.invocation != before {
		s.format = m.rational.decimalStream.invocation
	}
	if e != nil {
		return e
	}
	s.formatOwned = true
	return nil
}

type arbitraryFloatDecimalStreamScope struct {
	manager                             *ArbitraryFloatManager
	q                                   Rational
	conversion, format                  uint64
	conversionOwned, formatOwned, qLive bool
	qCursor                             uint8
	integerBinding, naturalBinding      NumericCallerBinding
	active, executing, finishing        bool
	bodyCause, sourceCause              error
}

func (s *arbitraryFloatDecimalStreamScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.decimalStream
}
func (s *arbitraryFloatDecimalStreamScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatDecimalStreamScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 34); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 34); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatDecimalStreamScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}

func (m *ArbitraryFloatManager) finishDecimalStreamSource() error {
	s := &m.decimalStream
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
		if m.rational.decimalStream.invocation != s.format {
			return ErrNumericOwner
		}
		if e := m.rational.decimalStream.sourceCause; e != nil {
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

func (m *ArbitraryFloatManager) CleanupDecimalStream() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.decimalStream
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
		if m.rational.decimalStream.invocation != s.format {
			return ErrNumericOwner
		}
		if e := m.rational.CleanupDecimalStream(); e != nil {
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
