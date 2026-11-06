package numeral

// Source: src/util/mpf.cpp:186-192; src/util/mpf.h:80.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type ArbitraryFloatPlainRationalSetReceipt struct {
	manager    *ArbitraryFloatManager
	frame      *arbitraryFloatPlainRationalSetScope
	invocation uint64
	child      ArbitraryFloatRationalSetReceipt
}
type arbitraryFloatPlainRationalSetScope struct {
	manager                               *ArbitraryFloatManager
	invocation                            uint64
	exp                                   Int
	expLive, active, executing, finishing bool
	child                                 ArbitraryFloatRationalSetReceipt
	childFrame                            *arbitraryFloatRationalSetScope
	childInvocation                       uint64
	receipt                               *ArbitraryFloatPlainRationalSetReceipt
	bodyCause, sourceCause, ownCause      error
}

func (m *ArbitraryFloatManager) SetRational(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, value *Rational) error {
	return m.SetRationalWithReceipt(out, ebits, sbits, mode, value, nil)
}
func (m *ArbitraryFloatManager) SetRationalWithReceipt(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, value *Rational, r *ArbitraryFloatPlainRationalSetReceipt) (cause error) {
	return m.setRationalWithBeginReceipt(out, ebits, sbits, mode, value, r, nil, nil)
}
func (m *ArbitraryFloatManager) setRationalWithBeginReceipt(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, value *Rational, r *ArbitraryFloatPlainRationalSetReceipt, frame **arbitraryFloatPlainRationalSetScope, invocation *uint64) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	s := &m.plainRationalSet
	if s.executing || s.finishing || s.active || s.invocation == ^uint64(0) || m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	*s = arbitraryFloatPlainRationalSetScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, receipt: r}
	if frame != nil {
		*frame = s
	}
	if invocation != nil {
		*invocation = s.invocation
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = errors.Join(cause, m.finishPlainRationalSetSource())
		s.recordPlainHistory()
	}()
	if e := m.traceSetPlainRationalEntry(value, ebits, sbits); e != nil {
		return e
	}
	// Literal scoped_mpz declaration and SET0, before the reached child.
	s.expLive = true
	if e := m.integer.SetInt64(&s.exp, 0); e != nil {
		return e
	}
	if e := m.setExponentRationalWithPlainCaller(out, ebits, sbits, mode, &s.exp, value, &s.child, s); e != nil {
		return e
	}
	return m.traceSetOrdinaryResult(out, 191)
}
func (s *arbitraryFloatPlainRationalSetScope) matched(m *ArbitraryFloatManager) bool {
	return m != nil && m.self == m && s == &m.plainRationalSet && s.manager == m && s.invocation != 0
}
func (s *arbitraryFloatPlainRationalSetScope) recordPlainHistory() {
	if !s.matched(s.manager) {
		return
	}
	if h := s.child.frame; h != nil && h.Self == h && h.Manager == s.manager && h.plainParent == s && h.plainInvocation == s.invocation {
		h.plainBodyCause, h.plainSourceCause, h.plainOwnCause = s.bodyCause, s.sourceCause, s.ownCause
		h.plainExpFinished = !s.expLive
	}
}
func (m *ArbitraryFloatManager) plainRationalChildSource(s *arbitraryFloatPlainRationalSetScope) error {
	if s.child.frame != nil {
		_, e, _ := m.ExponentRationalSetInvocationCauses(&s.child)
		if e != nil {
			return e
		}
	}
	if s.childFrame == &m.rationalSet && s.childInvocation != 0 && s.childInvocation == m.rationalSet.invocation {
		return m.rationalSet.sourceCause
	}
	return nil
}
func (m *ArbitraryFloatManager) finishPlainRationalSetSource() error {
	s := &m.plainRationalSet
	if !s.matched(m) {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	if s.executing || s.finishing || m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	s.finishing = true
	defer func() { s.finishing = false; s.recordPlainHistory() }()
	if e := m.plainRationalChildSource(s); e != nil {
		s.sourceCause = e
		return e
	}
	if s.expLive {
		if _, e := sourceNumeralDel(m.integer, &s.exp); e != nil {
			s.sourceCause = e
			return e
		}
		s.expLive = false
	}
	s.active = false
	return nil
}
func (m *ArbitraryFloatManager) CleanupPlainRationalSet() error {
	if e := m.CleanupPlainRationalSetWithReceipt(nil, false); e != nil {
		return e
	}
	return m.CleanupPlainRationalSetWithReceipt(nil, true)
}
func (m *ArbitraryFloatManager) CleanupPlainRationalSetWithReceipt(r *ArbitraryFloatPlainRationalSetReceipt, abort bool) error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.plainRationalSet
	if s.executing || s.finishing || m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if r != nil {
		if *r == (ArbitraryFloatPlainRationalSetReceipt{}) {
			return nil
		}
		if r.manager != m || r.frame != s || r.invocation == 0 {
			return umemory.ErrRuntimeOwner
		}
		h, e := m.rationalSetHistory(&r.child)
		if e != nil {
			return e
		}
		if h == nil || h.plainParent != s || h.plainInvocation != r.invocation {
			return umemory.ErrRuntimeOwner
		}
		if s.invocation != r.invocation {
			if !h.plainExpFinished {
				return ErrNumericPending
			}
			e = m.CleanupExponentRationalSetWithReceipt(&r.child, abort)
			if e != nil && h.plainOwnCause == nil {
				h.plainOwnCause = e
			}
			return e
		}
	}
	if !s.matched(m) {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false; s.recordPlainHistory() }()
	stop := func(e error) error {
		if e != nil && s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	if e := m.CleanupExponentRationalSetWithReceipt(&s.child, false); e != nil {
		return stop(e)
	}
	if s.expLive {
		if e := m.integer.Del(&s.exp); e != nil {
			return stop(e)
		}
		s.expLive = false
	}
	if abort {
		if e := m.CleanupExponentRationalSetWithReceipt(&s.child, true); e != nil {
			return stop(e)
		}
	}
	s.active = false
	return nil
}
func (m *ArbitraryFloatManager) PlainRationalSetInvocationCauses(r *ArbitraryFloatPlainRationalSetReceipt) (error, error, error) {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame != &m.plainRationalSet || r.invocation == 0 {
		return nil, nil, nil
	}
	h, e := m.rationalSetHistory(&r.child)
	if e != nil || h == nil || h.plainParent != r.frame || h.plainInvocation != r.invocation {
		return nil, nil, nil
	}
	return h.plainBodyCause, h.plainSourceCause, h.plainOwnCause
}
