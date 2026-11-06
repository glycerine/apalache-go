package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1019-1299.

import (
	"reflect"
)

type upolySquareFreeDiagnosticHistory struct {
	self                             *upolySquareFreeDiagnosticHistory
	service                          *UPolynomialFactorizationService
	receipt                          upolyFactorPreparationReceipt
	next                             *upolySquareFreeDiagnosticHistory
	header                           uint64
	bodyCause, sourceCause, ownCause error
}

type upolyFactorSquareFreeInvocationReceipt struct {
	entry      upolyDisplayInvocationReceipt
	history    *upolySquareFreeDiagnosticHistory
	service    *UPolynomialFactorizationService
	frame      *upolyFactorSquareFreeScope
	invocation uint64
	diagnostic upolyFactorPreparationReceipt
}

func (s *UPolynomialFactorizationService) matchesDiagnosticHistory(r *upolyFactorSquareFreeInvocationReceipt) bool {
	h := r.history
	if h == nil || h.self != h || h.service != s || h.header == 0 || h.receipt != r.diagnostic || h.receipt.frame != &s.diagnostic || h.receipt.service != s || h.receipt.invocation == 0 {
		return false
	}
	for old := s.squareFreeDiagnosticHistory; old != nil; old = old.next {
		if old == h {
			return true
		}
	}
	return false
}
func (s *UPolynomialFactorizationService) matchesSquareFreeHistory(r *upolyFactorSquareFreeInvocationReceipt) bool {
	if s == nil || s.self != s || s.closed || s.manager == nil || s.manager.self != s.manager || r == nil || r.service != s {
		return false
	}
	if r.frame == nil {
		return r.invocation == 0 && (s.matchesDiagnosticHistory(r) || s.manager.matchesDisplayReceipt(&r.entry))
	}
	c := r.frame
	if *r != c.issuedReceipt {
		return false
	}
	if c.self != c || c.service != s || c.manager != s.manager || r.invocation == 0 || c.invocation != r.invocation {
		return false
	}
	if s.scope == c && !c.retired {
		return true
	}
	if !c.retired || c.header == 0 {
		return false
	}
	for old := s.squareFreeHistory; old != nil; old = old.historyNext {
		if old == c {
			return true
		}
	}
	return false
}
func (s *UPolynomialFactorizationService) factorSquareFreeInvocationSourceCause(r *upolyFactorSquareFreeInvocationReceipt) error {
	if !s.matchesSquareFreeHistory(r) {
		return nil
	}
	if s.manager.displayReceiptPending(&r.entry) {
		if e := s.manager.displayReceiptSourceCause(&r.entry); e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if s.matchesDiagnosticHistory(r) && r.history.sourceCause != nil {
		return r.history.sourceCause
	}
	if r.frame != nil {
		if e := r.frame.traceSourceBlocker(); e != nil {
			return e
		}
		return r.frame.sourceCause
	}
	return nil
}
func (s *UPolynomialFactorizationService) releaseSquareFreeScope(c *upolyFactorSquareFreeScope) {
	c.retired = true
	if c.historyRequired {
		c.historyNext = s.squareFreeHistory
		s.squareFreeHistory = c
	} else {
		s.manager.config.Integer.Budget().Release(c.header)
		c.header = 0
	}
	if s.scope == c {
		s.scope = nil
	}
}
func (s *UPolynomialFactorizationService) captureSquareFreeDiagnostic(q *upolyFactorPreparationScope, r *upolyFactorSquareFreeInvocationReceipt) error {
	if r == nil {
		return nil
	}
	n := uint64(reflect.TypeOf(upolySquareFreeDiagnosticHistory{}).Size())
	if e := s.manager.config.Integer.Budget().Reserve(n); e != nil {
		return e
	}
	h := &upolySquareFreeDiagnosticHistory{service: s, receipt: q.receipt, header: n, next: s.squareFreeDiagnosticHistory}
	h.self = h
	s.squareFreeDiagnosticHistory = h
	*r = upolyFactorSquareFreeInvocationReceipt{service: s, diagnostic: q.receipt, history: h, entry: r.entry}
	return nil
}
func (s *UPolynomialFactorizationService) cleanupFactorSquareFreeInvocation(r *upolyFactorSquareFreeInvocationReceipt, abort bool) (cause error) {
	if r == nil || r.service == nil {
		return nil
	}
	if !s.matchesSquareFreeHistory(r) {
		return ErrUPolynomialOwner
	}
	if s.entryTraceBorrow.executing {
		return ErrUPolynomialCleanupPending
	}
	if e := s.manager.cleanupDisplayReceipt(&r.entry); e != nil {
		return e
	}
	c := r.frame
	if c != nil && (c.executing || c.sourceExecuting || c.ownExecuting || c.callDepth != 0) {
		return ErrUPolynomialCleanupPending
	}
	if s.diagnostic.matches(&r.diagnostic) {
		q := &s.diagnostic
		if q.executing || q.sourceExecuting || q.ownExecuting || q.callDepth != 0 {
			return ErrUPolynomialCleanupPending
		}
	}
	if c != nil && !c.retired {
		if s.scope != c {
			return ErrUPolynomialOwner
		}
		c.ownExecuting = true
		defer func() { c.ownExecuting = false }()
	}
	defer func() {
		if cause != nil {
			if c != nil && c.ownCause == nil {
				c.ownCause = cause
			}
			if s.matchesDiagnosticHistory(r) && r.history.ownCause == nil {
				r.history.ownCause = cause
			}
		}
	}()
	if s.diagnostic.matches(&r.diagnostic) {
		if e := s.diagnostic.own(&r.diagnostic, false); e != nil {
			return e
		}
	}
	if c != nil && !c.retired {
		if s.scope != c {
			return ErrUPolynomialOwner
		}
		if q := &s.preparation; q.active && q.parent == c {
			if e := q.own(&q.receipt, false); e != nil {
				return e
			}
		}
		if e := s.cleanupSquareFreeMembers(c, false); e != nil {
			return e
		}
		if abort {
			if e := s.cleanupSquareFreeMembers(c, true); e != nil {
				return e
			}
			s.releaseSquareFreeScope(c)
		}
	}
	if abort && s.diagnostic.matches(&r.diagnostic) {
		return s.diagnostic.own(&r.diagnostic, true)
	}
	return nil
}

func (c *upolyFactorSquareFreeScope) squareFreeChildSourceCause() error {
	if q := &c.primePreparation; q.matches() && q.sourceCause != nil {
		return q.sourceCause
	}
	if q := &c.service.preparation; q.parent == c && q.matches(&q.receipt) && q.sourceCause != nil {
		return q.sourceCause
	}
	if q := &c.recombinationStep; q.sourceCause != nil {
		return q.sourceCause
	}
	if q := &c.trialHelper; q.sourceCause != nil {
		return q.sourceCause
	}
	if q := &c.hensel; q.sourceCause != nil {
		return q.sourceCause
	}
	if q := &c.primeTrial; q.sourceCause != nil {
		return q.sourceCause
	}
	return c.mignotte.sourceCause
}
