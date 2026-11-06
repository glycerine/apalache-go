package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:440-444;
// scoped_numeral.h, mpq.h::del (NUM before DEN).

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The source scoped_mpq tmp uses the existing localQ payload. Its literal
// destructor and explicit owning continuation never share a destruction cursor.
type algebraicConstQScope struct {
	owner                               *AlgebraicRootCore
	invocation                          uint64
	copy                                unumeral.IntegerCopyAllocationReceipt
	sourceCursor, ownCursor             uint8
	active, running, executing          bool
	bodyCause, sourceStop, cleanupCause error
}

func (m *AlgebraicRootCore) beginConstQ() error {
	s := &m.constQ
	if s.active || s.running || s.executing || s.invocation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	if e := m.beginQ(); e != nil {
		return e
	}
	*s = algebraicConstQScope{owner: m, invocation: s.invocation + 1, active: true, running: true}
	return nil
}
func (s *algebraicConstQScope) valid(m *AlgebraicRootCore) bool {
	return m != nil && m.self == m && !m.closed && s == &m.constQ && s.owner == m && m.localActive
}
func (m *AlgebraicRootCore) finishConstQ(cause error) error {
	s := &m.constQ
	if !s.active {
		return cause
	}
	if !s.valid(m) {
		return errors.Join(cause, ErrAlgebraicOwner)
	}
	if s.running || s.executing {
		return errors.Join(cause, ErrAlgebraicPending)
	}
	s.executing = true
	defer func() { s.executing = false }()
	if s.bodyCause == nil {
		s.bodyCause = cause
	}
	if s.sourceStop != nil {
		return errors.Join(cause, s.sourceStop)
	}
	m.localOriginal = cause
	z := m.config.Rationals.Integers()
	fields := [2]*unumeral.Int{m.localQ.BorrowNumerator(), m.localQ.BorrowDenominator()}
	for s.sourceCursor < 2 {
		if e := unumeral.SourceIntegerDel(z, fields[s.sourceCursor]); e != nil {
			s.sourceStop = e
			m.localCleanup = e
			return errors.Join(cause, e)
		}
		s.sourceCursor++
	}
	if !z.MatchesCopyAllocationInvocation(&s.copy) {
		s.active = false
		m.localActive = false
		m.localOriginal = nil
		m.localCleanup = nil
	}
	return cause
}
func (m *AlgebraicRootCore) cleanupConstQ() error {
	s := &m.constQ
	if !s.active {
		return nil
	}
	if !s.valid(m) {
		return ErrAlgebraicOwner
	}
	if s.running || s.executing {
		return ErrAlgebraicPending
	}
	s.executing = true
	defer func() { s.executing = false }()
	e := m.cleanupConstQBody()
	if e != nil && s.cleanupCause == nil {
		s.cleanupCause = e
	}
	return e
}
func (m *AlgebraicRootCore) cleanupConstQBody() error {
	s := &m.constQ
	z := m.config.Rationals.Integers()
	if e := z.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	fields := [2]*unumeral.Int{m.localQ.BorrowNumerator(), m.localQ.BorrowDenominator()}
	for s.ownCursor < 2 {
		if e := z.CleanupRawNumeral(fields[s.ownCursor]); e != nil {
			return e
		}
		s.ownCursor++
	}
	s.active = false
	m.localActive = false
	m.localOriginal = nil
	m.localCleanup = nil
	return nil
}

func (m *AlgebraicRootCore) copyConstQ(q *unumeral.Rational) error {
	s := &m.constQ
	if !s.valid(m) || !s.active {
		return ErrAlgebraicOwner
	}
	if !s.running || s.executing {
		return ErrAlgebraicPending
	}
	s.executing = true
	defer func() {
		s.executing = false
		// This is the actual accepted copy receipt, not an allocator snapshot.
		// Later mutable-destination copies retain their independent core entry.
		m.rationalCopy = s.copy
	}()
	return m.config.Rationals.CopyWithAllocationReceipt(&m.localQ, q, &s.copy)
}
