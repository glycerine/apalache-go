package polynomial

import (
	"errors"
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Each actual caller holds its own accepted insertion receipts. The default
// call retains its original lexical NUM array only if source destruction stops.
type upolyCRATrace struct {
	invocation                              uint64
	parent                                  *upolySquareFreeCallScope
	parentInvocation                        uint64
	frontend                                *upolyFactorFrontendScope
	owner                                   *UPolynomialManager
	insertions                              [2]unumeral.NumeralInsertionReceipt
	executing                               bool
	values                                  *[12]unumeral.Int
	declared, sourceRemaining, ownRemaining uint8
	bodyCause, sourceCause                  error
}

// A failed insertion body can leave an unjoined receipt after its source
// destructors finished. Admit only the real parent source-unwind reads, while
// keeping output/runtime callbacks and new ordinary work blocked.
func (q *upolyCRATrace) sourceReadAllowed() bool {
	f := q.frontend
	if f == nil || !f.actual() || !f.active || !f.sourceExecuting || q != &f.cra.trace {
		return false
	}
	m := q.owner
	return m.callbackDepth == 0 && !m.config.Integer.RuntimeOwner().FailureTransportActive() && !m.config.Integer.BorrowTraceTags().OutputCallbackActive()
}
func (q *upolyCRATrace) pending() bool {
	return q != nil && (q.insertions[0].Active() || q.insertions[1].Active() || q.ownRemaining != 0)
}
func (q *upolyCRATrace) sourceFailure() error {
	if q == nil {
		return nil
	}
	if q.sourceCause != nil {
		return q.sourceCause
	}
	if q.parent != nil && q.parent.valid() && q.parentInvocation == q.parent.invocation {
		if e := q.parent.numericSourceCause(); e != nil {
			return e
		}
	}
	for i := range q.insertions {
		if e := q.insertions[i].SourceCause(); e != nil {
			return e
		}
	}
	return nil
}
func (q *upolyCRATrace) cleanup() error {
	if q == nil || q.owner == nil {
		return nil
	}
	m := q.owner
	if q.executing || m.factorTrace != nil && m.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	for i := len(q.insertions) - 1; i >= 0; i-- {
		r := &q.insertions[i]
		if r.Active() {
			if e := r.Complete(m.config.Integer.Naturals(), false); e != nil {
				return e
			}
			if e := r.Complete(m.config.Integer.Naturals(), true); e != nil {
				return e
			}
		}
	}
	for q.ownRemaining > 0 {
		if e := m.config.Integer.CleanupRawNumeral(&q.values[q.ownRemaining-1]); e != nil {
			return e
		}
		q.ownRemaining--
	}
	q.values = nil
	if m.pendingCRATrace == q && !q.pending() {
		m.pendingCRATrace = nil
	}
	return nil
}
func (m *UPolynomialManager) CleanupCRATrace() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if m.factorTrace != nil && m.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	return m.craTrace.cleanup()
}
func (m *UPolynomialManager) traceCRAPair(q *upolyCRATrace, line int, a, b *unumeral.Int) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("CRA") {
		return nil
	}
	if q == nil || q.owner != m {
		return ErrUPolynomialOwner
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	labels := [2]string{"inv1: ", ", inv2: "}
	if line == 682 {
		labels = [2]string{"a1: ", ", a2: "}
	} else if line == 692 {
		labels = [2]string{"lower: ", ", upper: "}
	}
	var computation error
	_ = tags.EmitConditional("CRA", "CRA_combine_images", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i, n := range [2]*unumeral.Int{a, b} {
			_ = linearWriteText(out, labels[i])
			if e := m.config.Integer.InsertSourceIntegerTracked(factorizationTraceStream{out}, n, &q.insertions[i], nil); e != nil {
				computation = e
				return e
			}
		}
		_ = linearWriteText(out, "\n")
		return nil
	})
	if q.pending() {
		m.pendingCRATrace = q
	}
	return computation
}
func (q *upolyCRATrace) finishDefault(values *[12]unumeral.Int, body error) error {
	q.executing = false
	q.bodyCause = body
	q.sourceRemaining = q.declared
	if q.sourceFailure() == nil {
		for q.sourceRemaining > 0 {
			if _, e := upolynomialSourceNumeralDel(q.owner.config.Integer, &values[q.sourceRemaining-1]); e != nil {
				q.sourceCause = e
				break
			}
			q.sourceRemaining--
		}
	}
	if q.sourceRemaining == 0 {
		q.values = nil
	}
	if q.sourceRemaining != 0 {
		q.values = values
		q.ownRemaining = q.sourceRemaining
	}
	if q.pending() {
		q.owner.pendingCRATrace = q
	}
	if e := q.sourceFailure(); e != nil {
		return errors.Join(body, e)
	}
	return body
}
