package polynomial

// Source: src/math/polynomial/polynomial.cpp:2396-2440; src/math/polynomial/polynomial.h:226-240.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// PolynomialDeletionCall is one named, static cold dispatch. The core borrows
// handlers; neither terms nor the physical polynomial contain boxed callbacks.
type PolynomialDeletionCall func(*PolynomialDeletionHandler, *MultivariatePolynomialCore, Polynomial) error

type PolynomialDeletionHandler struct {
	self   *PolynomialDeletionHandler
	next   *PolynomialDeletionHandler
	core   *MultivariatePolynomialCore
	budget *arena.Budget
	call   PolynomialDeletionCall
	header uint64
	State  uint64 // Caller-defined scalar dispatch data, not an owning numeral.
	closed bool
}

func NewPolynomialDeletionHandler(b *arena.Budget, call PolynomialDeletionCall) (*PolynomialDeletionHandler, error) {
	if b == nil || call == nil {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(PolynomialDeletionHandler{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	h := &PolynomialDeletionHandler{budget: b, call: call, header: n}
	h.self = h
	return h, nil
}
func (h *PolynomialDeletionHandler) check() error {
	if h == nil || h.self != h {
		return ErrPolynomialOwner
	}
	if h.closed {
		return arena.ErrClosed
	}
	return nil
}
func (h *PolynomialDeletionHandler) CloseChecked() error {
	if h == nil {
		return nil
	}
	if h.self != h {
		return ErrPolynomialOwner
	}
	if h.closed {
		return nil
	}
	if h.core != nil && !h.core.closed {
		return ErrPolynomialLive
	}
	h.core = nil
	h.budget.Release(h.header)
	h.header = 0
	h.closed = true
	return nil
}
func (m *MultivariatePolynomialCore) deletionListReady() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if m.callbackDepth == 0 {
		return m.check()
	}
	return nil
}
func (m *MultivariatePolynomialCore) AddDeletionHandler(h *PolynomialDeletionHandler) error {
	if e := m.deletionListReady(); e != nil {
		return e
	}
	if e := h.check(); e != nil {
		return e
	}
	// Borrowed-owner safety prevents double registration/cyclic ownership; the
	// source caller likewise must keep every registered event handler alive.
	if h.core != nil {
		return ErrPolynomialLive
	}
	if h.budget != m.integer.Budget() {
		return arena.ErrConfig
	}
	h.next = m.deletionHead
	m.deletionHead = h
	h.core = m
	return nil
}
func (m *MultivariatePolynomialCore) RemoveDeletionHandler(h *PolynomialDeletionHandler) error {
	if e := m.deletionListReady(); e != nil {
		return e
	}
	if polynomialSourceDebug && (h == nil || m.deletionHead == nil) {
		return ErrPolynomialAssertion
	}
	if h == nil || m.deletionHead == nil {
		return ErrPolynomialOwner
	}
	if m.deletionHead == h {
		if e := h.check(); e != nil {
			return e
		}
		m.deletionHead = h.next
		h.core = nil
		return nil
	}
	for curr := m.deletionHead; curr != nil; {
		if e := curr.check(); e != nil {
			return e
		}
		next := curr.next
		if next == h {
			if e := h.check(); e != nil {
				return e
			}
			curr.next = h.next
			h.core = nil
			return nil
		}
		curr = next
	}
	return ErrPolynomialAssertion // Typed continuation at source UNREACHABLE.
}
func (m *MultivariatePolynomialCore) deletionCallbacks(p Polynomial) error {
	l, e := m.polynomialLocation(p)
	if e != nil {
		return e
	}
	if l.callbackCause != nil {
		return l.callbackCause
	}
	if !l.callbackStarted {
		l.callbackStarted = true
		l.callback = m.deletionHead
	}
	for l.callback != nil {
		h := l.callback
		if e = h.check(); e != nil {
			l.callbackCause = e
			return e
		}
		if !l.callbackInvoked {
			// A callback may create/delete peers and thereby move the cold directory.
			// Reacquire the actual scalar polynomial locator after returning.
			m.callbackDepth++
			old := m.callbackValue
			m.callbackValue = p
			s := m.compositionOwner
			executing := s != nil && s.executing
			if executing {
				s.executing = false
			}
			e = h.call(h, m, p)
			if executing {
				s.executing = true
			}
			m.callbackValue = old
			m.callbackDepth--
			var lookup error
			l, lookup = m.polynomialLocation(p)
			if lookup != nil {
				return lookup
			}
			l.callbackInvoked = true
			if e != nil {
				l.callbackCause = e
				return e
			}
		}
		// The source reads current->next only AFTER the successful call, so removal
		// or insertion during that call is observed here. A dead current is guarded
		// at this access, not before its previously completed invocation.
		if e = h.check(); e != nil {
			l.callbackCause = e
			return e
		}
		l.callback = h.next
		l.callbackInvoked = false
	}
	return nil
}

// CleanupValue is an owning abort for this actual caller-reachable value. It
// skips remaining callbacks after a stopped source deletion, without claiming
// native exception recovery or discovering roots from the locator directory.
func (m *MultivariatePolynomialCore) CleanupValue(p *Polynomial) error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if p == nil {
		return arena.ErrBounds
	}
	if *p == (Polynomial{}) {
		return nil
	}
	if m.callbackDepth != 0 && m.callbackValue == *p {
		return ErrPolynomialPending
	}
	l, e := m.polynomialLocation(*p)
	if e != nil {
		return e
	}
	if l.deletionRunning {
		return ErrPolynomialPending
	}
	if l.deleteStage == 0 {
		l.deleteStage = 1
	}
	e = m.deleteCoreMode(*p, true)
	if !l.live {
		*p = Polynomial{}
	}
	return e
}
