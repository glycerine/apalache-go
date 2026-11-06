package polynomial

// Source: upolynomial.cpp:2823-2827, before either scoped binary bound exists.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

type upolyConvertEntryReceipt struct {
	signs            [2]upolyRationalEvaluationInvocationReceipt
	boundsInvocation uint64
	owner            *UPolynomialManager
	invocation       uint64
}
type upolyConvertEntryScope struct {
	owner                     *UPolynomialManager
	binary                    *unumeral.BinaryRationalManager
	invocation                uint64
	active, executing, owning bool
	insertions                [4]unumeral.NumeralInsertionReceipt
	borrow                    upolyFactorTraceBorrow
	binaryBinding             unumeral.NumericCallerBinding
	bodyCause, sourceCause    error
}

func (q *upolyConvertEntryScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q == &q.owner.convertEntry && q.active && (q.executing || q.owning)
}
func (q *upolyConvertEntryScope) NumericBinaryExecuting(b *unumeral.BinaryRationalManager) bool {
	return q.NumericExecuting() && q.binary == b
}
func (q *upolyConvertEntryScope) pending() bool {
	for i := range q.insertions {
		if q.insertions[i].Active() {
			return true
		}
	}
	return false
}
func (m *UPolynomialManager) convertEntryReceiptPending(r *upolyConvertEntryReceipt) bool {
	if r == nil || r.owner != m {
		return false
	}
	for i := range r.signs {
		if m.matchesRationalEvaluationReceipt(&r.signs[i]) && m.rationalEvaluation.active {
			return true
		}
	}
	return (r.invocation != 0 && r.invocation == m.convertEntry.invocation && m.convertEntry.active) || (r.boundsInvocation != 0 && r.boundsInvocation == m.intervalBounds.invocation && m.intervalBounds.active)
}
func (m *UPolynomialManager) convertEntryReceiptSourceCause(r *upolyConvertEntryReceipt) error {
	if r == nil || r.owner != m {
		return nil
	}
	for i := range r.signs {
		if m.matchesRationalEvaluationReceipt(&r.signs[i]) && m.rationalEvaluation.sourceCause != nil {
			return m.rationalEvaluation.sourceCause
		}
	}
	if r.boundsInvocation != 0 && r.boundsInvocation == m.intervalBounds.invocation && m.intervalBounds.active {
		return m.intervalBounds.sourceCause
	}
	if r.invocation != 0 && r.invocation == m.convertEntry.invocation && m.convertEntry.active {
		return m.convertEntry.sourceCause
	}
	return nil
}
func (m *UPolynomialManager) cleanupConvertEntryReceipt(r *upolyConvertEntryReceipt) error {
	if r == nil || r.owner != m {
		return nil
	}
	for i := len(r.signs) - 1; i >= 0; i-- {
		if e := m.cleanupRationalEvaluationWithReceipt(&r.signs[i], false); e != nil {
			return e
		}
		if e := m.cleanupRationalEvaluationWithReceipt(&r.signs[i], true); e != nil {
			return e
		}
	}
	if r.boundsInvocation != 0 && r.boundsInvocation == m.intervalBounds.invocation && m.intervalBounds.active {
		if e := m.cleanupIntervalBounds(); e != nil {
			return e
		}
	}
	if r.invocation != 0 && r.invocation == m.convertEntry.invocation && m.convertEntry.active {
		return m.CleanupConversionEntryTrace()
	}
	return nil
}
func (m *UPolynomialManager) traceConvertEntry(bqm *unumeral.BinaryRationalManager, a, b *unumeral.Rational, sa, sb int, receipt *upolyConvertEntryReceipt) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("convert_bug") {
		return nil
	}
	q := &m.convertEntry
	if q.active || q.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*q = upolyConvertEntryScope{owner: m, binary: bqm, invocation: q.invocation + 1, active: true, executing: true}
	if e := q.borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		q.active = false
		q.executing = false
		return e
	}
	defer q.borrow.unbind()
	if e := m.config.Integer.BindCaller(&q.binaryBinding, q, 0); e != nil {
		q.active = false
		q.executing = false
		return e
	}
	defer func() {
		if e := q.binaryBinding.Unbind(); e != nil {
			panic(e)
		}
	}()
	if receipt != nil {
		receipt.owner = m
		receipt.invocation = q.invocation
	}
	defer func() {
		q.executing = false
		for i := range q.insertions {
			if q.sourceCause == nil {
				q.sourceCause = q.insertions[i].SourceCause()
			}
		}
		q.active = q.pending()
	}()
	var computation error
	_ = tags.EmitConditional("convert_bug", "convert_q2bq_interval", "src/math/polynomial/upolynomial.cpp", 2823, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i, n := range [4]*unumeral.Int{a.BorrowNumerator(), a.BorrowDenominator(), b.BorrowNumerator(), b.BorrowDenominator()} {
			_ = linearWriteText(out, [4]string{"a: ", "/", "\nb: ", "/"}[i])
			text, e := m.modular.SourceStringTracked(n, &q.insertions[i])
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, text)
		}
		_ = linearWriteText(out, "\nsign_a: ")
		_ = linearWriteText(out, strconv.Itoa(sa))
		_ = linearWriteText(out, "\nsign_b: ")
		_ = linearWriteText(out, strconv.Itoa(sb))
		_ = linearWriteText(out, "\n")
		return nil
	})
	q.bodyCause = computation
	return computation
}

// Only the actual accepted diagnostic children belong to this owning adapter.
// No binary-bound constructor or numerical conversion is represented here.
func (m *UPolynomialManager) CleanupConversionEntryTrace() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.convertEntry
	if !q.active {
		return nil
	}
	if q.owner != m {
		return ErrUPolynomialOwner
	}
	if q.executing || q.owning || m.factorTrace != nil && m.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	q.owning = true
	defer func() { q.owning = false }()
	if e := q.borrow.bindManagers([3]*UPolynomialManager{m}, m.modular.BorrowTraceTags()); e != nil {
		return e
	}
	defer q.borrow.unbind()
	if e := m.config.Integer.BindCaller(&q.binaryBinding, q, 0); e != nil {
		return e
	}
	defer func() {
		if e := q.binaryBinding.Unbind(); e != nil {
			panic(e)
		}
	}()
	for i := len(q.insertions) - 1; i >= 0; i-- {
		if e := m.modular.CleanupSourceString(&q.insertions[i]); e != nil {
			return e
		}
	}
	q.active = false
	return nil
}
