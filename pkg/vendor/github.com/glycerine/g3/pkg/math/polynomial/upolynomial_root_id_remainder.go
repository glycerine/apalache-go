package polynomial

// Source: upolynomial.cpp:516-567,2520-2546, reached by get_root_id.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolySturmRemainderScalar struct {
	value                            unumeral.Int
	copy                             unumeral.IntegerCopyAllocationReceipt
	normalize                        unumeral.IntegerNormalizeInvocationReceipt
	addMul                           unumeral.IntegerAddMulInvocationReceipt
	invocation                       uint64
	constructed                      bool
	sourceCursor, ownCursor          uint8
	bodyCause, sourceStop, childStop error
}

func (c integerAllocationInvocationContext) rootIDFrame(z *unumeral.IntegerManager) *upolyRootIDScope {
	q := c.rootID
	if q == nil || q.owner == nil || q.owner.self != q.owner || q != &q.owner.rootID || q.owner.config.Integer != z || !q.active || !q.executing || q.finishing || q.owner.callbackDepth != 0 || q.owner.config.Integer.RuntimeOwner().FailureTransportActive() || c.rootIDInvocation == 0 || c.rootIDInvocation != q.invocation {
		return nil
	}
	return q
}
func (q *upolyRootIDScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{rootID: q, rootIDInvocation: q.invocation}
}
func (q *upolyRootIDScope) NumericBound(executing bool) bool {
	m := q.owner
	return m != nil && m.self == m && !m.closed && q == &m.rootID && q.active && (!executing || q.executing && !q.finishing && m.callbackDepth == 0 && !m.config.Integer.RuntimeOwner().FailureTransportActive())
}
func (q *upolyRootIDScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !q.NumericBound(true) || q.owner.config.Integer != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &q.numeric
}
func (q *upolyRootIDScope) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if q.NumericBound(true) && q.owner.config.Integer == z && r.Raw != nil {
		q.raw = append(q.raw, *r.Raw)
	}
}
func (q *upolyRootIDScope) scalarPending() bool {
	z := q.owner.config.Integer
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
func (q *upolyRootIDScope) beginRemainderScalar() error {
	if q.scalarPending() || q.scalar.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	q.scalar = upolySturmRemainderScalar{invocation: q.scalar.invocation + 1, constructed: true}
	return nil
}
func (q *upolyRootIDScope) finishRemainderScalar(cause error) error {
	s := &q.scalar
	z := q.owner.config.Integer
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
		if e := unumeral.SourceIntegerDel(z, &s.value); e != nil {
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
func (q *upolyRootIDScope) cleanupRemainderNumerics() error {
	m, z := q.owner, q.owner.config.Integer
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
	if e := z.CleanupCopyAllocationInvocation(&q.remainderCopy); e != nil {
		return e
	}
	return nil
}
