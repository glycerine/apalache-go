package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:749-753,775-779;
// src/util/scoped_numeral.h and mpq.h del: numerator before denominator.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The numeral itself is the existing core.localQ. This scope retains the
// actual source destructor cursor and separate owning continuation state.
type algebraicRefinementQScope struct {
	owner                                          *AlgebraicRootCore
	sourceCursor, ownCursor                        uint8
	active, executing                              bool
	bodyCause, childStop, sourceStop, cleanupCause error
}

func (m *AlgebraicRootCore) beginRefinementQ() error {
	s := &m.refinementQ
	if s.active || s.executing {
		return ErrAlgebraicPending
	}
	if e := m.beginQ(); e != nil {
		return e
	}
	*s = algebraicRefinementQScope{owner: m, active: true}
	return nil
}
func (s *algebraicRefinementQScope) valid(m *AlgebraicRootCore) bool {
	return m != nil && m.self == m && s == &m.refinementQ && s.owner == m && m.localActive
}
func (s *algebraicRefinementQScope) finish(m *AlgebraicRootCore, cause error) error {
	if !s.active {
		return cause
	}
	if !s.valid(m) {
		return errors.Join(cause, ErrAlgebraicOwner)
	}
	if s.executing {
		return errors.Join(cause, ErrAlgebraicPending)
	}
	s.executing = true
	defer func() { s.executing = false }()
	if s.bodyCause == nil {
		s.bodyCause = cause
	}
	if s.childStop == nil {
		s.childStop = m.refinementConversion.stop
	}
	if s.childStop != nil || s.sourceStop != nil {
		return errors.Join(cause, s.childStop, s.sourceStop)
	}
	z := m.config.Rationals.Integers()
	fields := [2]*unumeral.Int{m.localQ.BorrowNumerator(), m.localQ.BorrowDenominator()}
	for s.sourceCursor < 2 {
		if e := unumeral.SourceIntegerDel(z, fields[s.sourceCursor]); e != nil {
			s.sourceStop = e
			m.localOriginal = cause
			m.localCleanup = e
			return errors.Join(cause, e)
		}
		s.sourceCursor++
	}
	s.active = false
	m.localActive = false
	m.localOriginal = nil
	m.localCleanup = nil
	return cause
}
func (s *algebraicRefinementQScope) cleanup(m *AlgebraicRootCore) error {
	if !s.active {
		return nil
	}
	if !s.valid(m) {
		return ErrAlgebraicOwner
	}
	if s.executing {
		return ErrAlgebraicPending
	}
	s.executing = true
	defer func() { s.executing = false }()
	z := m.config.Rationals.Integers()
	fields := [2]*unumeral.Int{m.localQ.BorrowNumerator(), m.localQ.BorrowDenominator()}
	for s.ownCursor < 2 {
		if e := z.CleanupRawNumeral(fields[s.ownCursor]); e != nil {
			if s.cleanupCause == nil {
				s.cleanupCause = e
			}
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
