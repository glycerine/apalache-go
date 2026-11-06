package numeral

// Source: src/util/mpf.cpp:194-231; src/util/mpq.cpp rational text SET;
// src/util/mpz.cpp:283-303.

import (
	"strings"

	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// SetFormattedLiteral parses the source [i].[f]P[e] spelling. The suffix is a
// power of two; the rational parser retains responsibility for decimal syntax.
func (m *ArbitraryFloatManager) SetFormattedLiteral(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, value string) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	s := &m.formattedLiteral
	if s.active || s.executing || s.finishing || s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatFormattedLiteralScope{manager: m, invocation: s.invocation + 1, active: true, executing: true}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		if cause == nil {
			if e := s.cleanupChild(); e != nil {
				cause = e
			} else {
				s.childOwned = true
			}
		}
		cause = JoinNumericErrors(cause, m.finishFormattedLiteralSource())
		if cause == nil {
			cause = m.CleanupFormattedLiteral()
		}
	}()
	m.traceSetLiteralEntry(value, ebits, sbits)

	// These source writes precede all string operations and numeric declarations.
	out.assignEbits(ebits)
	out.assignSbits(sbits)
	if i := strings.IndexByte(value, 0); i >= 0 {
		value = value[:i]
	}
	negative := false
	if strings.HasPrefix(value, "-") {
		negative = true
		value = value[1:]
	} else if strings.HasPrefix(value, "+") {
		value = value[1:]
	}
	// The source deliberately looks for any lowercase p before any uppercase P.
	pos := strings.IndexByte(value, 'p')
	if pos < 0 {
		pos = strings.IndexByte(value, 'P')
	}
	fraction, exponent := value, "0"
	if pos >= 0 {
		fraction, exponent = value[:pos], value[pos+1:]
	}
	m.traceSetLiteralParts(negative, fraction, exponent)

	s.q = NewRational()
	s.qLive = true
	if e := m.rational.SetString(&s.q, fraction); e != nil {
		return e
	}
	s.exLive = true
	beforeParser := m.integer.stringSet.invocation
	e := m.integer.SetString(&s.ex, exponent)
	if m.integer.stringSet.invocation != beforeParser {
		s.parserInvocation = m.integer.stringSet.invocation
	}
	if e != nil {
		return e
	}
	if s.parserInvocation != 0 {
		if e := m.integer.CleanupStringSet(); e != nil {
			return e
		}
		s.parserOwned = true
	}
	beforeChild := m.rationalSet.invocation
	e = m.SetExponentRationalWithReceipt(out, ebits, sbits, mode, &s.ex, &s.q, &s.child)
	if m.rationalSet.invocation != beforeChild {
		s.childFrame = &m.rationalSet
		s.childInvocation = m.rationalSet.invocation
	}
	if e != nil {
		return e
	}
	out.assignSign(negative)
	return m.traceSetOrdinaryResult(out, 230)
}

type arbitraryFloatFormattedLiteralScope struct {
	manager                          *ArbitraryFloatManager
	invocation                       uint64
	q                                Rational
	ex                               Int
	qLive, exLive                    bool
	qCursor                          uint8
	parserInvocation                 uint64
	parserOwned                      bool
	child                            ArbitraryFloatRationalSetReceipt
	childFrame                       *arbitraryFloatRationalSetScope
	childInvocation                  uint64
	childOwned                       bool
	active, executing, finishing     bool
	bodyCause, sourceCause, ownCause error
}

func (s *arbitraryFloatFormattedLiteralScope) cleanupChild() error {
	if s.childOwned || s.childFrame == nil {
		return nil
	}
	m := s.manager
	if s.childFrame != &m.rationalSet || s.childInvocation == 0 || s.childFrame.invocation != s.childInvocation {
		return ErrNumericOwner
	}
	if s.child.frame != nil {
		return m.CleanupExponentRationalSetWithReceipt(&s.child, true)
	}
	return m.cleanupExponentRationalSetScope(nil, true, true)
}

func (m *ArbitraryFloatManager) finishFormattedLiteralSource() error {
	s := &m.formattedLiteral
	if !s.active {
		return nil
	}
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if s.childFrame == &m.rationalSet && s.childInvocation == s.childFrame.invocation && s.childFrame.sourceCause != nil {
		s.sourceCause = s.childFrame.sourceCause
		return s.sourceCause
	}
	if s.parserInvocation != 0 && s.parserInvocation == m.integer.stringSet.invocation && m.integer.stringSet.sourceCause != nil {
		s.sourceCause = m.integer.stringSet.sourceCause
		return s.sourceCause
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if s.exLive {
		if _, e := SourceNumeralDel(m.integer, &s.ex); e != nil {
			s.sourceCause = e
			return e
		}
		s.exLive = false
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

// CleanupFormattedLiteral joins the exact reached child and owned lexical
// numerals. It does not replay a source destructor stopped by an earlier error.
func (m *ArbitraryFloatManager) CleanupFormattedLiteral() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.formattedLiteral
	if !s.active {
		return nil
	}
	if s.executing || s.finishing || m.memory != nil && m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if s.manager != m || s.invocation == 0 {
		return ErrNumericOwner
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	stop := func(e error) error {
		if e != nil && s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	if !s.childOwned {
		if e := s.cleanupChild(); e != nil {
			return stop(e)
		}
		s.childOwned = true
	}
	if s.parserInvocation != 0 && !s.parserOwned {
		if s.parserInvocation != m.integer.stringSet.invocation {
			return stop(ErrNumericOwner)
		}
		if e := m.integer.CleanupStringSet(); e != nil {
			return stop(e)
		}
		s.parserOwned = true
	}
	if s.exLive {
		if e := m.integer.CleanupRawNumeral(&s.ex); e != nil {
			return stop(e)
		}
		s.exLive = false
	}
	for s.qLive && s.qCursor < 2 {
		v := &s.q.Num
		if s.qCursor == 1 {
			v = &s.q.Den
		}
		if e := m.integer.CleanupRawNumeral(v); e != nil {
			return stop(e)
		}
		s.qCursor++
	}
	s.qLive = false
	s.active = false
	return nil
}
