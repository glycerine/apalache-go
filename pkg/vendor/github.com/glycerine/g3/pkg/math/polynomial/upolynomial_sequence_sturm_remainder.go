package polynomial

// Source: upolynomial.cpp:516-574,2520-2559; public sequence remainder locals.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (q *upolySequenceSturmScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{sequenceSturm: q, sequenceSturmInvocation: q.invocation}
}
func (c integerAllocationInvocationContext) sequenceSturmFrame(z *unumeral.IntegerManager) *upolySequenceSturmScope {
	q := c.sequenceSturm
	if q == nil || !q.NumericBound(true) || q.owner.manager.config.Integer != z || c.sequenceSturmInvocation == 0 || c.sequenceSturmInvocation != q.invocation {
		return nil
	}
	return q
}
func (q *upolySequenceSturmScope) NumericBound(executing bool) bool {
	return q.valid() && !q.owner.closed && !q.owner.manager.closed && q.active && (!executing || q.executing && !q.finishing && q.currentNumericOwner() && q.owner.manager.callbackDepth == 0 && !q.owner.manager.config.Integer.RuntimeOwner().FailureTransportActive())
}
func (q *upolySequenceSturmScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !q.NumericBound(true) || q.owner.manager.config.Integer != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &q.numeric
}
func (q *upolySequenceSturmScope) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if q.NumericBound(true) && q.owner.manager.config.Integer == z && r.Raw != nil {
		q.raw = append(q.raw, *r.Raw)
	}
}
func (q *upolySequenceSturmScope) scalarPending() bool {
	z := q.owner.manager.config.Integer
	if q.scalar.constructed || z.MatchesCopyAllocationInvocation(&q.scalar.copy) || z.MatchesAddMulInvocation(&q.scalar.addMul) {
		return true
	}
	for i := range q.raw {
		if z.MatchesRawInvocation(&q.raw[i]) {
			return true
		}
	}
	return false
}
func (q *upolySequenceSturmScope) remainderNumericPending() bool {
	m := q.owner.manager
	return q.scalarPending() || unumeral.NumericAllocationPending(&q.numeric) || m.config.Integer.MatchesCopyAllocationInvocation(&q.remainderCopy) || m.modular.MatchesPNormalizeInvocation(&q.remainderNormalize)
}
func (q *upolySequenceSturmScope) beginRemainderScalar() error {
	if q.scalarPending() || q.scalar.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	q.scalar = upolySturmRemainderScalar{invocation: q.scalar.invocation + 1, constructed: true}
	return nil
}
func (q *upolySequenceSturmScope) finishRemainderScalar(cause error) error {
	s, z := &q.scalar, q.owner.manager.config.Integer
	if s.bodyCause == nil {
		s.bodyCause = cause
	}
	if s.childStop == nil {
		s.childStop = q.remainderTraceSourceCause()
	}
	if s.childStop == nil {
		s.childStop = z.AddMulInvocationSourceCause(&s.addMul)
	}
	for i := len(q.raw) - 1; i >= 0 && s.childStop == nil; i-- {
		s.childStop = z.RawInvocationSourceCause(&q.raw[i])
	}
	if s.childStop == nil && s.sourceStop == nil && s.constructed {
		q.finishing = true
		e := unumeral.SourceIntegerDel(z, &s.value)
		q.finishing = false
		if e != nil {
			s.sourceStop = e
		} else {
			s.sourceCursor = 1
			s.constructed = false
		}
	}
	stop := errors.Join(s.childStop, s.sourceStop)
	if stop != nil && q.childStop == nil {
		q.childStop = stop
	}
	return errors.Join(cause, stop)
}
func (q *upolySequenceSturmScope) cleanupRemainderNumerics() error {
	m, z := q.owner.manager, q.owner.manager.config.Integer
	if e := q.numeric.Cleanup(z); e != nil {
		return e
	}
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := z.CleanupRawInvocation(&q.raw[i], false); e != nil {
			return e
		}
	}
	if e := q.cleanupNormalization(); e != nil {
		return e
	}
	if e := q.cleanupRemainderTrace(false); e != nil {
		return e
	}
	s := &q.scalar
	if e := z.CleanupAddMulInvocation(&s.addMul, false); e != nil {
		return e
	}
	if e := m.modular.CleanupPNormalizeInvocation(&s.normalize, true); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	if s.constructed {
		if e := z.CleanupRawNumeral(&s.value); e != nil {
			return e
		}
		s.ownCursor = 1
		s.constructed = false
	}
	if e := q.cleanupRemainderTrace(true); e != nil {
		return e
	}
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := z.CleanupRawInvocation(&q.raw[i], true); e != nil {
			return e
		}
	}
	if e := z.CleanupAddMulInvocation(&s.addMul, true); e != nil {
		return e
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.remainderNormalize, true); e != nil {
		return e
	}
	return z.CleanupCopyAllocationInvocation(&q.remainderCopy)
}

// The numeric caller chain also identifies this polynomial manager's actual
// public constructor, before manager member destruction can touch its buffers.
func (m *UPolynomialManager) sequenceSturmExecuting() bool {
	if m == nil || m.self != m || m.config.Integer == nil {
		return false
	}
	for b := m.config.Integer.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 17 {
			if q, ok := b.Guard().(*upolySequenceSturmScope); ok && q.NumericExecuting() && q.owner.manager == m {
				return true
			}
		}
	}
	return false
}

// Retain only actually pending public invocations so source manager teardown
// cannot invalidate the manager needed to acknowledge those exact children.
func (q *upolySequenceSturmScope) retainPending() {
	if q.retained {
		return
	}
	m := q.owner.manager
	q.next = m.sturmPending
	m.sturmPending = q
	q.retained = true
}
func (q *upolySequenceSturmScope) releasePending() {
	if !q.retained {
		return
	}
	link := &q.owner.manager.sturmPending
	for *link != nil {
		if *link == q {
			*link = q.next
			q.next = nil
			q.retained = false
			return
		}
		link = &(*link).next
	}
}
func (m *UPolynomialManager) pendingSturmSourceCause() error {
	for q := m.sturmPending; q != nil; q = q.next {
		if !q.valid() || q.owner.manager != m || !q.retained || !q.active {
			return ErrUPolynomialOwner
		}
		if q.sourceStop != nil || q.childStop != nil {
			return errors.Join(q.sourceStop, q.childStop)
		}
		return ErrUPolynomialCleanupPending
	}
	return nil
}

func (q *upolySequenceSturmScope) remainderTracePending() bool {
	return q.remainderTrace.pending(q.owner.manager)
}
func (q *upolySequenceSturmScope) remainderTraceSourceCause() error {
	return q.remainderTrace.sourceCause(q.owner.manager)
}
func (q *upolySequenceSturmScope) cleanupRemainderTrace(abort bool) error {
	return q.remainderTrace.cleanup(q.owner.manager, abort)
}
func (q *upolySequenceSturmScope) traceRemainder(phase uint8, b UPolynomialView, r *upolyVectorState, bn *unumeral.Int) error {
	m := q.owner.manager
	if !m.config.Integer.BorrowTraceTags().ShouldTrace("rem_bug") {
		return nil
	}
	if q.context().sequenceSturmFrame(m.config.Integer) != q || !q.scalar.constructed {
		return ErrUPolynomialOwner
	}
	if q.remainderTracePending() {
		return ErrUPolynomialCleanupPending
	}
	return q.remainderTrace.emit(m, phase, b, r, bn, &q.scalar.value)
}
