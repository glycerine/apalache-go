package numeral

// Source: src/util/rational.cpp:75-83; src/util/rational.h:67.
// Leonardo de Moura (2006-09-18).

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (s *RationalService) releaseOuterLeaf(a *Int, source bool) error {
	if source && s.Manager == nil {
		s.sourceGuard = true
		s.sourceCause = ErrRationalServiceNoManager
		return s.sourceCause
	}
	if !source {
		return s.Manager.Z.Del(a)
	}
	guard, e := sourceNumeralDel(s.Manager.Z, a)
	if guard {
		s.sourceGuard = true
		s.sourceCause = e
	}
	return e
}
func (s *RationalService) SourceFinalize() error {
	if s == nil || s.self != s {
		return ErrRationalServiceCopy
	}
	if s.sourceInsertionExecuting || s.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sourceFinalizeLocked()
}

// Caller owns the existing service mutex; owning cleanup can resume without
// recursively acquiring a public lock or returning through a destroyed child.
func (s *RationalService) sourceFinalizeLocked() error {
	if s.closed || s.sourceFinalized {
		return nil
	}
	if s.closing && !s.sourceFinalizing {
		return arena.ErrClosed
	}
	if e := s.lexicalInvocationSourceReady(); e != nil {
		return e
	}
	if s.Lexical.Pending() {
		return ErrRationalLexicalPending
	}
	s.sourceFinalizing = true
	if s.sourceGuard || s.sourceGlobalStopped {
		return s.sourceCause
	}
	fail := func(e error) error {
		if s.sourceGlobalSlot != nil {
			s.sourceGlobalStopped = true
		}
		s.sourceCause = e
		s.cleanupCause = e
		return e
	}
	if e := s.cleanupOuter(true); e != nil {
		return fail(e)
	}
	if e := s.Manager.SourceFinalize(); e != nil {
		if s.Manager.sourceGuard {
			s.sourceGuard = true
		}
		return fail(e)
	}
	if s.sourceGlobalSlot != nil {
		*s.sourceGlobalSlot = nil
	}
	s.sourceFinalized = true
	s.sourceCause = nil
	s.cleanupCause = nil
	return nil
}
