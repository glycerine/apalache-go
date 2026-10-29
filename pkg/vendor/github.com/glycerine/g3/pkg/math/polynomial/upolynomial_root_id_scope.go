package polynomial

// Source: upolynomial.cpp:2520-2546,2951-2958; scoped_upolynomial_sequence.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyRootIDScope struct {
	remainderTrace                             upolySturmRemainderTrace
	normalization                              upolySturmNormalizeScope
	derivative                                 unumeral.Int
	derivativeRetired                          *upolySturmDerivativeRaw
	scalar                                     upolySturmRemainderScalar
	hooks                                      unumeral.NumericInvocationHooks
	numeric                                    unumeral.NumericAllocationRoots
	raw                                        []unumeral.IntegerRawInvocationReceipt
	remainderCopy                              unumeral.IntegerCopyAllocationReceipt
	remainderNormalize                         unumeral.IntegerNormalizeInvocationReceipt
	owner                                      *UPolynomialManager
	invocation                                 uint64
	target                                     UPolynomialView
	sequence                                   upolySequenceState
	prime, remainder                           upolyVectorState
	live                                       [3]bool // remainder, prime, sequence coefficients
	sourceCursor, ownCursor                    [3]uint32
	initialCopy                                unumeral.IntegerCopyAllocationReceipt
	normalize                                  unumeral.IntegerNormalizeInvocationReceipt
	evaluation                                 upolyBinaryEvaluationInvocationReceipt
	active, executing, finishing               bool
	bodyCause, sourceStop, childStop, ownCause error
}
type upolyRootIDInvocationReceipt struct {
	owner      *UPolynomialManager
	frame      *upolyRootIDScope
	invocation uint64
}

func (m *UPolynomialManager) rootIDExecuting() bool {
	return m != nil && m.self == m && m.rootID.owner == m && m.rootID.active && (m.rootID.executing || m.rootID.finishing)
}
func (m *UPolynomialManager) matchesRootID(r *upolyRootIDInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.frame == &m.rootID && m.rootID.owner == m && r.invocation != 0 && r.invocation == m.rootID.invocation
}
func (q *upolyRootIDScope) vector(which uint8) *upolyVectorState {
	if which == 0 {
		return &q.remainder
	}
	if which == 1 {
		return &q.prime
	}
	return &q.sequence.coefficients
}
func (q *upolyRootIDScope) pending() bool {
	m := q.owner
	if q.remainderTracePending() || q.normalizePending() || q.derivative.NativeCellToken() != 0 || q.derivativeRetired != nil || q.scalarPending() || unumeral.NumericAllocationPending(&q.numeric) || m.config.Integer.MatchesCopyAllocationInvocation(&q.remainderCopy) || q.live[0] || q.live[1] || q.live[2] || m.config.Integer.MatchesCopyAllocationInvocation(&q.initialCopy) || m.modular.MatchesPNormalizeInvocation(&q.normalize) {
		return true
	}
	return m.matchesBinaryEvaluationReceipt(&q.evaluation) && m.binaryEvaluation.active
}
func (q *upolyRootIDScope) finishVector(which uint8, cause error) error {
	if q.bodyCause == nil {
		q.bodyCause = cause
	}
	if q.sourceStop != nil || q.childStop != nil {
		return errors.Join(cause, q.sourceStop, q.childStop)
	}
	if !q.live[which] {
		return cause
	}
	m := q.owner
	q.finishing = true
	defer func() { q.finishing = false }()
	state := q.vector(which)
	a := m.values(state)
	for q.sourceCursor[which] < state.size {
		if e := unumeral.SourceIntegerDel(m.config.Integer, &a[q.sourceCursor[which]]); e != nil {
			q.sourceStop = e
			return errors.Join(cause, e)
		}
		q.sourceCursor[which]++
	}
	state.size = 0
	m.freeBacking(state)
	q.live[which] = false
	if which == 2 {
		q.sequence.begins = nil
		q.sequence.sizes = nil
		if !q.pending() {
			q.active = false
		}
	}
	return cause
}
func (q *upolyRootIDScope) sturm(p UPolynomialView) (cause error) {
	defer func() { cause = q.finishVector(1, cause) }()
	if e := q.sturmPrefix(p); e != nil {
		return e
	}
	return q.sturmCore()
}
func (q *upolyRootIDScope) sturmPrefix(p UPolynomialView) error {
	m := q.owner
	if e := m.sequenceReset(&q.sequence); e != nil {
		return e
	}
	q.live[1] = true
	// Source seq.push(m(),sz,p): record only the accepted initial copies.
	if e := upolyUnsignedPush(&q.sequence.begins, q.sequence.coefficients.size); e != nil {
		return e
	}
	if e := upolyUnsignedPush(&q.sequence.sizes, p.count); e != nil {
		return e
	}
	for i := uint32(0); i < p.count; i++ {
		dst, e := m.pushDefault(&q.sequence.coefficients)
		if e != nil {
			return e
		}
		src, e := m.read(p, i)
		if e != nil {
			return e
		}
		if e = m.modular.CopyFromOwnerTracked(dst, m.config.Integer, src, &q.initialCopy, &q.normalize); e != nil {
			return e
		}
	}
	if e := m.derivativeStateWithSourceContext(p, &q.prime, q.context()); e != nil {
		return e
	}
	if e := m.sequencePush(&q.sequence, m.stateView(&q.prime), true); e != nil {
		return e
	}
	return nil
}
func (q *upolyRootIDScope) sturmCore() (cause error) {
	m := q.owner
	q.live[0] = true
	defer func() { cause = q.finishVector(0, cause) }()
	for {
		n := uint32(len(q.sequence.sizes))
		a, e := m.sequenceRow(&q.sequence, n-2)
		if e != nil {
			return e
		}
		b, e := m.sequenceRow(&q.sequence, n-1)
		if e != nil {
			return e
		}
		if e = m.signedRemStateWithSourceContext(a, b, &q.remainder, q.context()); e != nil {
			return e
		}
		if q.remainder.size == 0 {
			return nil
		}
		if e = q.normalizeView(m.stateView(&q.remainder)); e != nil {
			return e
		}
		if e = m.sequencePush(&q.sequence, m.stateView(&q.remainder), true); e != nil {
			return e
		}
	}
}
func (m *UPolynomialManager) CleanupRootID() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	r := upolyRootIDInvocationReceipt{m, &m.rootID, m.rootID.invocation}
	return m.cleanupRootIDWithReceipt(&r)
}
func (m *UPolynomialManager) cleanupRootIDWithReceipt(r *upolyRootIDInvocationReceipt) (cause error) {
	if !m.matchesRootID(r) {
		return nil
	}
	q := &m.rootID
	if !q.active {
		return nil
	}
	if q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	defer func() {
		q.finishing = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	if e := m.cleanupBinaryEvaluationWithReceipt(&q.evaluation, false); e != nil {
		return e
	}
	if e := m.cleanupBinaryEvaluationWithReceipt(&q.evaluation, true); e != nil {
		return e
	}
	if e := q.cleanupRemainderNumerics(); e != nil {
		return e
	}
	if e := q.cleanupDerivativeMultiplier(); e != nil {
		return e
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return e
	}
	if e := m.config.Integer.CleanupCopyAllocationInvocation(&q.initialCopy); e != nil {
		return e
	}
	for which := uint8(0); which < 3; which++ {
		if !q.live[which] {
			continue
		}
		state := q.vector(which)
		a := m.values(state)
		for q.ownCursor[which] < state.size {
			if e := m.config.Integer.CleanupRawNumeral(&a[q.ownCursor[which]]); e != nil {
				return e
			}
			q.ownCursor[which]++
		}
		state.size = 0
		m.freeBacking(state)
		q.live[which] = false
	}
	q.sequence.begins = nil
	q.sequence.sizes = nil
	q.active = false
	return nil
}
