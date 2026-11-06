package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1238-1292;
// src/math/polynomial/upolynomial.cpp:213-232,609-651.
// Concrete call adapter only: this source span declares no additional numeral.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyRecombinationStepInvocationReceipt struct {
	owner      *upolyFactorSquareFreeScope
	frame      *upolyRecombinationStepScope
	invocation uint64
}
type upolyRecombinationStepScope struct {
	exactTrace                                       upolyDisplayInvocationReceipt
	receipt                                          upolyRecombinationStepInvocationReceipt
	callerPrevious                                   [2]*upolyRecombinationStepScope
	callDepth                                        uint32
	self                                             *upolyRecombinationStepScope
	owner                                            *upolyFactorSquareFreeScope
	factors                                          *UPolynomialFactors
	related                                          *UPolynomialManager
	invocation                                       uint64
	active, executing, sourceExecuting, ownExecuting bool
	sourceEnded, ownEnded                            bool
	numeric                                          polynomialNumericRoots
	normalize                                        [2]unumeral.IntegerNormalizeInvocationReceipt
	boolNumeral                                      unumeral.IntegerDividesNumeralInvocationReceipt
	addMul                                           unumeral.IntegerAddMulInvocationReceipt
	divisionRaw, extendedRaw, gcdRaw                 unumeral.IntegerRawInvocationReceipt

	bodyCause, sourceCause, ownCause error
}

func (q *upolyRecombinationStepScope) matches() bool {
	return q != nil && q.self == q && q.owner != nil && q.owner.self == q.owner && q == &q.owner.recombinationStep && q.owner.service != nil && q.owner.service.self == q.owner.service && q.owner.service.manager == q.owner.manager && q.owner.manager != nil && q.owner.manager.self == q.owner.manager && q.invocation != 0
}
func (s *UPolynomialFactorizationService) beginRecombinationStep(fs *UPolynomialFactors) (*upolyRecombinationStepScope, error) {
	if e := s.checkOwner(); e != nil {
		return nil, e
	}
	if s.scope == nil || s.scope.zpe == nil || s.scope.zpe.self != s.scope.zpe || s.scope.zpe.config.Integer != s.manager.config.Integer || fs == nil || fs.manager == nil || fs.manager.self != fs.manager || fs.manager.config.Integer != s.manager.config.Integer {
		return nil, ErrUPolynomialOwner
	}
	q := &s.scope.recombinationStep
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	*q = upolyRecombinationStepScope{owner: s.scope, factors: fs, related: s.scope.zpe, invocation: q.invocation + 1, active: true, executing: true}
	q.self = q
	q.receipt = upolyRecombinationStepInvocationReceipt{q.owner, q, q.invocation}
	return q, nil
}
func (q *upolyRecombinationStepScope) managers() [2]*UPolynomialManager {
	return [2]*UPolynomialManager{q.owner.manager, q.related}
}
func (q *upolyRecombinationStepScope) childCause() error {
	if e := q.exactTrace.sourceBlocker(); e != nil {
		return e
	}
	if e := q.owner.traceSourceBlocker(); e != nil {
		return e
	}
	z := q.owner.manager.config.Integer
	if e := z.DividesNumeralInvocationSourceCause(&q.boolNumeral); e != nil {
		return e
	}
	if e := z.AddMulInvocationSourceCause(&q.addMul); e != nil {
		return e
	}
	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.divisionRaw, &q.extendedRaw, &q.gcdRaw} {
		if e := z.RawInvocationSourceCause(r); e != nil {
			return e
		}
	}
	return nil
}
func (q *upolyRecombinationStepScope) matchesReceipt(r *upolyRecombinationStepInvocationReceipt) bool {
	return q.matches() && r != nil && r.owner == q.owner && r.frame == q && r.invocation != 0 && r.invocation == q.invocation
}
func (q *upolyRecombinationStepScope) finish(r *upolyRecombinationStepInvocationReceipt, body error) error {
	if !q.matchesReceipt(r) {
		return ErrUPolynomialOwner
	}
	if !q.active {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if q.owner.service.scope != q.owner {
		return ErrUPolynomialOwner
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
	q.sourceEnded = true

	if !q.pending() {
		q.active = false
	}
	return q.bodyCause
}
func (q *upolyRecombinationStepScope) pending() bool {
	if q.exactTrace.owner.displayReceiptPending(&q.exactTrace) {
		return true
	}
	if q.owner.tracePending() {
		return true
	}
	z := q.owner.manager.config.Integer
	if polynomialNegNumericPending(&q.numeric) || z.MatchesDividesNumeralInvocation(&q.boolNumeral) || z.MatchesAddMulInvocation(&q.addMul) {
		return true
	}

	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.divisionRaw, &q.extendedRaw, &q.gcdRaw} {
		if z.MatchesRawInvocation(r) {
			return true
		}
	}

	for i, m := range q.managers() {
		if m.modular.MatchesPNormalizeInvocation(&q.normalize[i]) {
			return true
		}
	}
	return false
}
func (q *upolyRecombinationStepScope) own(r *upolyRecombinationStepInvocationReceipt, abort bool) (cause error) {
	if !q.matchesReceipt(r) {
		return ErrUPolynomialOwner
	}
	if !q.active {
		return nil
	}
	if q.owner.service.scope != q.owner {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	defer func() {
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	q.ownExecuting = true
	defer func() { q.ownExecuting = false }()
	q.enterCall()
	defer q.leaveCall()
	if e := q.exactTrace.cleanupOwned(); e != nil {
		return e
	}
	z := q.owner.manager.config.Integer
	for i, m := range q.managers() {
		if e := m.modular.CleanupPNormalizeInvocation(&q.normalize[i], false); e != nil {
			return e
		}
	}
	if e := z.CleanupDividesNumeralInvocation(&q.boolNumeral, false); e != nil {
		return e
	}
	if e := z.CleanupAddMulInvocation(&q.addMul, false); e != nil {
		return e
	}
	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.divisionRaw, &q.extendedRaw, &q.gcdRaw} {
		if e := z.CleanupRawInvocation(r, false); e != nil {
			return e
		}
	}

	q.ownEnded = true
	if abort {
		for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.divisionRaw, &q.extendedRaw, &q.gcdRaw} {
			if e := z.CleanupRawInvocation(r, true); e != nil {
				return e
			}
		}

		for i, m := range q.managers() {
			if e := m.modular.CleanupPNormalizeInvocation(&q.normalize[i], true); e != nil {
				return e
			}
		}
		if e := z.CleanupDividesNumeralInvocation(&q.boolNumeral, true); e != nil {
			return e
		}
		if e := z.CleanupAddMulInvocation(&q.addMul, true); e != nil {
			return e
		}
		if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
			return e
		}
		q.active = false
	}
	return nil
}

func (q *upolyRecombinationStepScope) enterCall() {
	if q.callDepth == 0 {
		for i, m := range q.managers() {
			if i == 1 && m == q.owner.manager {
				continue
			}
			q.callerPrevious[i] = m.recombinationCaller
			m.recombinationCaller = q
		}
	}
	q.callDepth++
	q.owner.enterCall()
}
func (q *upolyRecombinationStepScope) leaveCall() {
	q.owner.leaveCall()
	q.callDepth--
	if q.callDepth == 0 {
		for i, m := range q.managers() {
			if i == 1 && m == q.owner.manager {
				continue
			}
			m.recombinationCaller = q.callerPrevious[i]
			q.callerPrevious[i] = nil
		}
	}
}

func (m *UPolynomialManager) recombinationCallPending() bool {
	for q := m.recombinationCaller; q != nil; {
		if q.matches() && q.callDepth != 0 && (q.owner.manager == m || q.related == m) {
			return true
		}
		if q.owner != nil && q.owner.manager == m {
			q = q.callerPrevious[0]
		} else {
			q = q.callerPrevious[1]
		}
	}
	return false
}
