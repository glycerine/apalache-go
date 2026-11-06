package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1801-1820.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyIntegerEvaluationScope struct {
	constructed                              uint8
	target                                   UPolynomialView
	falseAcknowledged                        bool
	owner                                    *UPolynomialManager
	invocation                               uint64
	values                                   [1]unumeral.Int
	numeric                                  polynomialNumericRoots
	normalize                                unumeral.IntegerNormalizeInvocationReceipt
	addMul                                   unumeral.IntegerAddMulInvocationReceipt
	sourceRemaining, ownRemaining            uint8
	active, executing, finishing, ownStarted bool
	bodyCause, sourceCause, ownCause         error
	history                                  *upolyIntegerEvaluationHistory
}

// Diagnostics contain issued child tuples, never NUM payload/owning authority.
type upolyIntegerEvaluationHistory struct {
	constructed                      uint8
	owner                            *UPolynomialManager
	frame                            *upolyIntegerEvaluationScope
	next                             *upolyIntegerEvaluationHistory
	invocation                       uint64
	normalize                        unumeral.IntegerNormalizeInvocationReceipt
	addMul                           unumeral.IntegerAddMulInvocationReceipt
	bodyCause, sourceCause, ownCause error
	sourceRemaining                  uint8
}

func (m *UPolynomialManager) integerEvaluationExecuting() bool {
	return m != nil && m.self == m && m.integerEvaluation.owner == m && m.integerEvaluation.active && (m.integerEvaluation.executing || m.integerEvaluation.finishing)
}
func numericIntegerManager_integerEvaluationCallerExecuting(z *unumeral.IntegerManager) bool {
	return z != nil && z.ValidOwner() && integerEvaluationCaller(z) != nil && integerEvaluationCaller(z).config.Integer == z && integerEvaluationCaller(z).integerEvaluationExecuting()
}
func (c integerAllocationInvocationContext) integerEvaluationFrame(z *unumeral.IntegerManager) *upolyIntegerEvaluationScope {
	q := c.integerEvaluation
	if z == nil || !z.ValidOwner() || z.RuntimeOwner().FailureTransportActive() || q == nil || q.owner == nil || q.owner.self != q.owner || q != &q.owner.integerEvaluation || q.owner.config.Integer != z || integerEvaluationCaller(z) != q.owner || q.owner.callbackDepth != 0 || !q.active || !q.executing || c.integerEvaluationInvocation == 0 || c.integerEvaluationInvocation != q.invocation {
		return nil
	}
	if q.history != nil && (q.history.owner != q.owner || q.history.frame != q || q.history.invocation != q.invocation) {
		return nil
	}
	return q
}
func (q *upolyIntegerEvaluationScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{integerEvaluation: q, integerEvaluationInvocation: q.invocation}
}
func (q *upolyIntegerEvaluationScope) prepareHistory() error {
	if q.history != nil {
		return nil
	}
	n := uint64(reflect.TypeOf(upolyIntegerEvaluationHistory{}).Size())
	if e := q.owner.config.Integer.Budget().Reserve(n); e != nil {
		return e
	}
	q.history = &upolyIntegerEvaluationHistory{owner: q.owner, frame: q, next: q.owner.integerEvaluationHistory, invocation: q.invocation}
	q.owner.integerEvaluationHistory = q.history
	q.owner.integerEvaluationHistoryBytes += n
	q.updateHistory()
	return nil
}
func (q *upolyIntegerEvaluationScope) updateHistory() {
	if q.history != nil {
		h := q.history
		h.constructed = q.constructed
		h.normalize = q.normalize
		h.addMul = q.addMul
		h.bodyCause = q.bodyCause
		h.sourceCause = q.sourceCause
		h.ownCause = q.ownCause
		h.sourceRemaining = q.sourceRemaining
	}
}
func (q *upolyIntegerEvaluationScope) pending() bool {
	z := q.owner.config.Integer
	mm := q.owner.modular
	return q.sourceRemaining != 0 || polynomialNegNumericPending(&q.numeric) || mm.MatchesPNormalizeInvocation(&q.normalize) || z.MatchesAddMulInvocation(&q.addMul)
}
func (q *upolyIntegerEvaluationScope) finish(body error) error {
	if q.owner == nil || q.owner.self != q.owner || q != &q.owner.integerEvaluation || q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	defer func() { q.finishing = false; q.updateHistory() }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	z := q.owner.config.Integer
	if q.sourceCause == nil {
		q.sourceCause = z.AddMulInvocationSourceCause(&q.addMul)
	}
	if q.sourceCause == nil {
		for q.sourceRemaining != 0 {
			if _, e := upolynomialSourceNumeralDel(z, &q.values[q.sourceRemaining-1]); e != nil {
				q.sourceCause = e
				break
			}
			q.sourceRemaining--
		}
	}
	if !q.pending() {
		q.active = false
	}
	return errors.Join(body, q.sourceCause)
}

// CleanupIntegerEvaluation owns only the actual current public Horner scope.
// Completed younger children precede r, and unjoined children follow them.
func (m *UPolynomialManager) CleanupIntegerEvaluation() error {
	if e := m.cleanupIntegerEvaluation(false); e != nil {
		return e
	}
	return m.cleanupIntegerEvaluation(true)
}

type upolyIntegerEvaluationInvocationReceipt struct {
	owner      *UPolynomialManager
	frame      *upolyIntegerEvaluationScope
	invocation uint64
	history    *upolyIntegerEvaluationHistory
}

func (m *UPolynomialManager) integerEvaluationReceipt() upolyIntegerEvaluationInvocationReceipt {
	q := &m.integerEvaluation
	return upolyIntegerEvaluationInvocationReceipt{m, q, q.invocation, q.history}
}
func (m *UPolynomialManager) cleanupIntegerEvaluation(abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	r := m.integerEvaluationReceipt()
	return m.cleanupIntegerEvaluationWithReceipt(&r, abort)
}
func (m *UPolynomialManager) cleanupIntegerEvaluationWithReceipt(r *upolyIntegerEvaluationInvocationReceipt, abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.integerEvaluation
	if r == nil || r.owner != m || r.frame != q || r.invocation == 0 || r.invocation != q.invocation || r.history != q.history {
		return nil
	}
	if q.history != nil && (q.history.owner != m || q.history.frame != q) {
		return ErrUPolynomialOwner
	}
	if !q.active {
		return nil
	}
	if q.owner != m || q.invocation == 0 || q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	z := m.config.Integer
	old := integerEvaluationCaller(z)
	setIntegerEvaluationCaller(z, m)
	defer func() { setIntegerEvaluationCaller(z, old); q.finishing = false; q.updateHistory() }()
	stop := func(e error) error {
		if e != nil && q.ownCause == nil {
			q.ownCause = e
		}
		return e
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, false); e != nil {
		return stop(e)
	}
	if e := z.CleanupAddMulInvocation(&q.addMul, false); e != nil {
		return stop(e)
	}
	if !q.ownStarted {
		q.ownRemaining = q.constructed
		q.ownStarted = true
	}
	for q.ownRemaining != 0 {
		if e := z.CleanupRawNumeral(&q.values[q.ownRemaining-1]); e != nil {
			return stop(e)
		}
		q.ownRemaining--
	}
	q.falseAcknowledged = true
	if !abort {
		return nil
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return stop(e)
	}
	if e := z.CleanupAddMulInvocation(&q.addMul, true); e != nil {
		return stop(e)
	}
	if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
		return stop(e)
	}
	q.active = false
	return nil
}
func (m *UPolynomialManager) beginIntegerEvaluation() (*upolyIntegerEvaluationScope, error) {
	q := &m.integerEvaluation
	if q.active {
		return nil, ErrUPolynomialCleanupPending
	}
	if q.invocation == math.MaxUint64 {
		return nil, arena.ErrBounds
	}
	*q = upolyIntegerEvaluationScope{owner: m, invocation: q.invocation + 1, active: true, executing: true, sourceRemaining: 1, constructed: 1}
	return q, nil
}
