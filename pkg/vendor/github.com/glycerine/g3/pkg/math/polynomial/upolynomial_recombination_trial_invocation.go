package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1213-1240;
// src/util/scoped_numeral.h:31-33.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyRecombinationTrialInvocationReceipt struct {
	owner      *upolyFactorSquareFreeScope
	frame      *upolyRecombinationTrialScope
	invocation uint64
}
type upolyRecombinationTrialScope struct {
	receipt                                          upolyRecombinationTrialInvocationReceipt
	callerPrevious                                   [2]*upolyRecombinationTrialScope
	callDepth                                        uint32
	self                                             *upolyRecombinationTrialScope
	owner                                            *upolyFactorSquareFreeScope
	factors                                          *UPolynomialFactors
	invocation                                       uint64
	active, executing, sourceExecuting, ownExecuting bool
	constructed, sourceEnded, ownEnded               bool
	tmp                                              unumeral.Int
	numeric                                          polynomialNumericRoots
	normalize                                        [2]unumeral.IntegerNormalizeInvocationReceipt
	boolNumeral                                      unumeral.IntegerDividesNumeralInvocationReceipt
	addMul                                           unumeral.IntegerAddMulInvocationReceipt
	bodyCause, sourceCause, ownCause                 error
}

func (q *upolyRecombinationTrialScope) matches() bool {
	return q != nil && q.self == q && q.owner != nil && q.owner.self == q.owner && q == &q.owner.trialHelper && q.owner.service != nil && q.owner.service.self == q.owner.service && q.owner.service.manager == q.owner.manager && q.owner.manager != nil && q.owner.manager.self == q.owner.manager && q.invocation != 0
}
func (s *UPolynomialFactorizationService) beginRecombinationTrial(fs *UPolynomialFactors) (*upolyRecombinationTrialScope, error) {
	if e := s.checkOwner(); e != nil {
		return nil, e
	}
	if s.scope == nil || fs == nil || fs.manager == nil || fs.manager.self != fs.manager || fs.manager.config.Integer != s.manager.config.Integer {
		return nil, ErrUPolynomialOwner
	}
	q := &s.scope.trialHelper
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	*q = upolyRecombinationTrialScope{owner: s.scope, factors: fs, invocation: q.invocation + 1, active: true, executing: true, constructed: true}
	q.self = q
	q.receipt = upolyRecombinationTrialInvocationReceipt{q.owner, q, q.invocation}
	return q, nil
}
func (q *upolyRecombinationTrialScope) managers() [2]*UPolynomialManager {
	return [2]*UPolynomialManager{q.owner.manager, q.factors.manager}
}
func (q *upolyRecombinationTrialScope) childCause() error {
	z := q.owner.manager.config.Integer
	if e := z.DividesNumeralInvocationSourceCause(&q.boolNumeral); e != nil {
		return e
	}
	return z.AddMulInvocationSourceCause(&q.addMul)
}
func (q *upolyRecombinationTrialScope) matchesReceipt(r *upolyRecombinationTrialInvocationReceipt) bool {
	return q.matches() && r != nil && r.owner == q.owner && r.frame == q && r.invocation != 0 && r.invocation == q.invocation
}
func (q *upolyRecombinationTrialScope) finish(r *upolyRecombinationTrialInvocationReceipt, body error) error {
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
	if q.constructed && !q.sourceEnded {
		if _, e := upolynomialSourceNumeralDel(q.owner.manager.config.Integer, &q.tmp); e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.sourceEnded = true
	}
	if !q.pending() {
		q.active = false
	}
	return q.bodyCause
}
func (q *upolyRecombinationTrialScope) pending() bool {
	z := q.owner.manager.config.Integer
	if polynomialNegNumericPending(&q.numeric) || z.MatchesDividesNumeralInvocation(&q.boolNumeral) || z.MatchesAddMulInvocation(&q.addMul) {
		return true
	}
	for i, m := range q.managers() {
		if m.modular.MatchesPNormalizeInvocation(&q.normalize[i]) {
			return true
		}
	}
	return false
}
func (q *upolyRecombinationTrialScope) own(r *upolyRecombinationTrialInvocationReceipt, abort bool) (cause error) {
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
	if q.constructed && !q.ownEnded {
		if e := z.CleanupRawNumeral(&q.tmp); e != nil {
			return e
		}
		q.ownEnded = true
	}
	if abort {
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

func (q *upolyRecombinationTrialScope) enterCall() {
	if q.callDepth == 0 {
		for i, m := range q.managers() {
			if i == 1 && m == q.owner.manager {
				continue
			}
			q.callerPrevious[i] = m.trialCaller
			m.trialCaller = q
		}
	}
	q.callDepth++
	q.owner.enterCall()
}
func (q *upolyRecombinationTrialScope) leaveCall() {
	q.owner.leaveCall()
	q.callDepth--
	if q.callDepth == 0 {
		for i, m := range q.managers() {
			if i == 1 && m == q.owner.manager {
				continue
			}
			m.trialCaller = q.callerPrevious[i]
			q.callerPrevious[i] = nil
		}
	}
}

func (m *UPolynomialManager) trialCallPending() bool {
	for q := m.trialCaller; q != nil; {
		if q.matches() && q.callDepth != 0 && (q.owner.manager == m || q.factors.manager == m) {
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
