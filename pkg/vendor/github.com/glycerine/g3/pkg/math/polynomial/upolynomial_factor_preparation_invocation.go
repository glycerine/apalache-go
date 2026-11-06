package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1030-1050.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyFactorPreparationReceipt struct {
	service    *UPolynomialFactorizationService
	frame      *upolyFactorPreparationScope
	invocation uint64
}
type upolyFactorPreparationScope struct {
	self                                                                                 *upolyFactorPreparationScope
	service                                                                              *UPolynomialFactorizationService
	parent                                                                               *upolyFactorSquareFreeScope
	receipt                                                                              upolyFactorPreparationReceipt
	invocation                                                                           uint64
	active, executing, sourceExecuting, ownExecuting, constructed, sourceEnded, ownEnded bool
	numeral                                                                              unumeral.Int
	numeric                                                                              polynomialNumericRoots
	normalize                                                                            unumeral.IntegerNormalizeInvocationReceipt
	gcdRaw                                                                               unumeral.IntegerRawInvocationReceipt
	bodyCause, sourceCause, ownCause                                                     error
	previous                                                                             *upolyFactorPreparationScope
	callDepth                                                                            uint32
}

func (q *upolyFactorPreparationScope) matches(r *upolyFactorPreparationReceipt) bool {
	return q != nil && q.self == q && q.service != nil && q.service.self == q.service && q.service.manager != nil && q.service.manager.self == q.service.manager && (q == &q.service.diagnostic || q == &q.service.preparation) && r != nil && r.service == q.service && r.frame == q && r.invocation != 0 && r.invocation == q.invocation
}
func (s *UPolynomialFactorizationService) beginFactorPreparation(debug bool) (*upolyFactorPreparationScope, error) {
	if e := s.checkOwner(); e != nil {
		return nil, e
	}
	q := &s.preparation
	if debug {
		q = &s.diagnostic
		if s.scope != nil {
			return nil, ErrUPolynomialCleanupPending
		}
	} else if s.scope == nil || s.scope.self != s.scope {
		return nil, ErrUPolynomialOwner
	}
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	*q = upolyFactorPreparationScope{service: s, invocation: q.invocation + 1, active: true, executing: true}
	if !debug {
		q.parent = s.scope
	}
	q.self = q
	q.receipt = upolyFactorPreparationReceipt{s, q, q.invocation}
	return q, nil
}
func (q *upolyFactorPreparationScope) enterCall() {
	if q.callDepth == 0 {
		q.previous = q.service.manager.preparationCaller
		q.service.manager.preparationCaller = q
	}
	q.callDepth++
}
func (q *upolyFactorPreparationScope) leaveCall() {
	q.callDepth--
	if q.callDepth == 0 {
		q.service.manager.preparationCaller = q.previous
		q.previous = nil
	}
}
func (m *UPolynomialManager) preparationCallPending() bool {
	for q := m.preparationCaller; q != nil; q = q.previous {
		if q.matches(&q.receipt) && q.service.manager == m && q.callDepth != 0 {
			return true
		}
	}
	return false
}
func (q *upolyFactorPreparationScope) childCause() error {
	z := q.service.manager.config.Integer

	return z.RawInvocationSourceCause(&q.gcdRaw)
}
func (q *upolyFactorPreparationScope) pending() bool {
	z := q.service.manager.config.Integer
	return polynomialNegNumericPending(&q.numeric) || q.service.manager.modular.MatchesPNormalizeInvocation(&q.normalize) || z.MatchesRawInvocation(&q.gcdRaw)
}
func (q *upolyFactorPreparationScope) finish(r *upolyFactorPreparationReceipt, body error) error {
	if !q.matches(r) {
		return ErrUPolynomialOwner
	}
	if !q.active {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.enterCall()
	defer q.leaveCall()
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause == nil {
		q.sourceCause = q.childCause()
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if q.constructed {
		if _, e := upolynomialSourceNumeralDel(q.service.manager.config.Integer, &q.numeral); e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.constructed = false
	}
	q.sourceEnded = true
	if !q.pending() {
		q.active = false
	}
	return q.bodyCause
}
func (q *upolyFactorPreparationScope) own(r *upolyFactorPreparationReceipt, abort bool) (cause error) {
	if !q.matches(r) {
		return ErrUPolynomialOwner
	}
	if !q.active {
		return nil
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	defer func() {
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	q.enterCall()
	defer q.leaveCall()
	q.ownExecuting = true
	defer func() { q.ownExecuting = false }()
	z := q.service.manager.config.Integer
	if e := q.service.manager.modular.CleanupPNormalizeInvocation(&q.normalize, false); e != nil {
		return e
	}
	if e := z.CleanupRawInvocation(&q.gcdRaw, false); e != nil {
		return e
	}
	if q.constructed {
		if e := z.CleanupRawNumeral(&q.numeral); e != nil {
			return e
		}
		q.constructed = false
	}
	q.ownEnded = true
	if abort {
		if e := z.CleanupRawInvocation(&q.gcdRaw, true); e != nil {
			return e
		}
		if e := q.service.manager.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
			return e
		}
		if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
			return e
		}
		q.active = false
	}
	return nil
}
func (s *UPolynomialFactorizationService) prepareSquareFree(f UPolynomialView, fs *UPolynomialFactors, k uint32) (failure error) {
	q, e := s.beginFactorPreparation(false)
	if e != nil {
		return e
	}
	r := q.receipt
	q.enterCall()
	defer q.leaveCall()
	defer func() { q.executing = false; failure = q.finish(&r, failure) }()
	m := s.manager
	context := numericIntegerModularManager_preparationAllocationContext(m.modular, q)
	if e = m.setViewWithSourceContext(&q.parent.fpp, f, context); e != nil {
		return e
	}
	p := &q.parent.fpp
	if p.size == 0 {
		return nil
	}
	top, e := m.read(m.stateView(p), p.size-1)
	if e != nil {
		return e
	}
	negative, e := m.modular.IsNeg(top)
	if e != nil || !negative {
		return e
	}
	for i := uint32(0); i < p.size; i++ {
		a, e := m.read(m.stateView(p), i)
		if e != nil {
			return e
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Neg(a); e != nil {
			return e
		}
	}
	if k%2 == 0 {
		return nil
	}
	q.constructed = true
	if e = m.checkFactors(fs); e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(&q.numeral, &fs.constant); e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Neg(&q.numeral); e != nil {
		return e
	}
	if e = fs.check(); e != nil {
		return e
	}
	return fs.manager.modular.WithAllocationContext((context).utility(fs.manager.modular.Integers())).Copy(&fs.constant, &q.numeral)
}
func (s *UPolynomialFactorizationService) primitiveDiagnosticWithPreparation(f UPolynomialView) error {
	return s.primitiveDiagnosticWithPreparationReceipt(f, nil)
}
func (s *UPolynomialFactorizationService) primitiveDiagnosticWithPreparationReceipt(f UPolynomialView, receipt *upolyFactorSquareFreeInvocationReceipt) (failure error) {
	q, e := s.beginFactorPreparation(true)
	if e != nil {
		return e
	}
	if e := s.captureSquareFreeDiagnostic(q, receipt); e != nil {
		q.executing = false
		return q.finish(&q.receipt, e)
	}
	r := q.receipt
	q.enterCall()
	defer q.leaveCall()
	defer func() {
		q.executing = false
		failure = q.finish(&r, failure)
		if receipt != nil {
			receipt.history.bodyCause = q.bodyCause
			receipt.history.sourceCause = q.sourceCause
		}
	}()
	q.constructed = true
	m := s.manager
	context := numericIntegerModularManager_preparationAllocationContext(m.modular, q)
	if e = m.gcdSpanWithSourceContext(f, &q.numeral, context); e != nil {
		return e
	}
	// DEBUG_CODE computes content even when runtime assertions are disabled.
	// Only SASSERT's predicate and debugger action are gated here.
	enabled, e := factorHeaderAssertionsEnabled()
	if e != nil {
		return e
	}
	if !enabled || f.count == 0 {
		return nil
	}
	one, e := m.modular.IsOne(&q.numeral)
	if e != nil {
		return e
	}
	if !one {
		return m.factorizationAssertionFailure(1033, "f.size() == 0 || nm.is_one(f_cont)")
	}
	return nil
}
