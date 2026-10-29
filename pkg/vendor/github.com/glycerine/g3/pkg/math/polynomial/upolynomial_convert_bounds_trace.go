package polynomial

// Source: upolynomial.cpp:2828-2916, including trace sites2830 and2841.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

type upolyIntervalBoundsScope struct {
	conversion                                 unumeral.BinaryConvertInvocationReceipt
	refinement                                 unumeral.BinaryRefineInvocationReceipt
	owner                                      *UPolynomialManager
	binary                                     *unumeral.BinaryRationalManager
	invocation                                 uint64
	active, executing, sourceExecuting, owning bool
	constructed, ownRemaining                  uint8
	values                                     [2]unumeral.BinaryRational // lower, upper
	insertions                                 [4]unumeral.NumeralInsertionReceipt
	evaluation                                 upolyBinaryEvaluationInvocationReceipt
	copy                                       unumeral.IntegerCopyAllocationReceipt
	borrow                                     upolyFactorTraceBorrow
	binaryBinding                              unumeral.NumericCallerBinding
	bodyCause, sourceCause                     error
}

func (q *upolyIntervalBoundsScope) NumericExecuting() bool {
	return q != nil && q.owner != nil && q == &q.owner.intervalBounds && q.active && (q.executing || q.sourceExecuting || q.owning)
}
func (q *upolyIntervalBoundsScope) NumericBinaryExecuting(b *unumeral.BinaryRationalManager) bool {
	return q.NumericExecuting() && q.binary == b
}
func (m *UPolynomialManager) beginIntervalBounds(b *unumeral.BinaryRationalManager, r *upolyConvertEntryReceipt) (*upolyIntervalBoundsScope, error) {
	q := &m.intervalBounds
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	*q = upolyIntervalBoundsScope{owner: m, binary: b, invocation: q.invocation + 1, active: true, executing: true, constructed: 2}
	if e := q.borrow.bindManagers([3]*UPolynomialManager{m}, m.modular.BorrowTraceTags()); e != nil {
		q.active = false
		q.executing = false
		return nil, e
	}
	if e := m.config.Integer.BindCaller(&q.binaryBinding, q, 0); e != nil {
		q.borrow.unbind()
		q.active = false
		q.executing = false
		return nil, e
	}
	if r != nil {
		r.owner = m
		r.boundsInvocation = q.invocation
	}
	return q, nil
}
func (q *upolyIntervalBoundsScope) pending() bool {
	if q.binary.ConvertInvocationPending(&q.conversion) {
		return true
	}
	if q.binary.RefineInvocationPending(&q.refinement) {
		return true
	}
	if q.constructed != 0 || q.owner.config.Integer.MatchesCopyAllocationInvocation(&q.copy) {
		return true
	}
	if q.owner.matchesBinaryEvaluationReceipt(&q.evaluation) && q.owner.binaryEvaluation.active {
		return true
	}
	for i := range q.insertions {
		if q.insertions[i].Active() {
			return true
		}
	}
	return false
}
func (q *upolyIntervalBoundsScope) finish(body error) error {
	q.executing = false
	defer q.borrow.unbind()
	defer func() {
		if e := q.binaryBinding.Unbind(); e != nil {
			panic(e)
		}
	}()
	q.bodyCause = body
	if q.sourceCause == nil {
		q.sourceCause = q.binary.ConvertInvocationSourceCause(&q.conversion)
	}
	if q.sourceCause == nil {
		q.sourceCause = q.binary.RefineInvocationSourceCause(&q.refinement)
	}
	for i := range q.insertions {
		if q.sourceCause == nil {
			q.sourceCause = q.insertions[i].SourceCause()
		}
	}
	if q.sourceCause == nil && q.owner.matchesBinaryEvaluationReceipt(&q.evaluation) {
		q.sourceCause = q.owner.binaryEvaluation.sourceCause
	}
	if q.sourceCause == nil {
		q.sourceExecuting = true
		for q.constructed > 0 {
			if e := unumeral.SourceIntegerDel(q.owner.config.Integer, q.values[q.constructed-1].BorrowNumerator()); e != nil {
				q.sourceCause = e
				break
			}
			q.constructed--
		}
		q.sourceExecuting = false
	}
	q.ownRemaining = q.constructed
	if !q.pending() {
		q.active = false
	}
	return errors.Join(body, q.sourceCause)
}

// CleanupIntervalConversion owns the current conversion's entry diagnostic
// and lexical bounds. Captured caller receipts use the narrower helpers below.
func (m *UPolynomialManager) CleanupIntervalConversion() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if m.intervalBounds.active {
		if e := m.cleanupIntervalBounds(); e != nil {
			return e
		}
	}
	if m.convertEntry.active {
		return m.CleanupConversionEntryTrace()
	}
	return nil
}
func (m *UPolynomialManager) cleanupIntervalBounds() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.intervalBounds
	if !q.active {
		return nil
	}
	if q.owner != m {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.owning || m.factorTrace != nil && m.factorTrace.executing {
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
		if e := q.binary.CleanupSourceString(&q.insertions[i]); e != nil {
			return e
		}
	}
	if e := m.cleanupBinaryEvaluationWithReceipt(&q.evaluation, false); e != nil {
		return e
	}
	if e := m.cleanupBinaryEvaluationWithReceipt(&q.evaluation, true); e != nil {
		return e
	}
	if e := m.config.Integer.CleanupCopyAllocationInvocation(&q.copy); e != nil {
		return e
	}
	if e := q.binary.CleanupConvertInvocation(&q.conversion); e != nil {
		return e
	}
	if e := q.binary.CleanupRefineInvocation(&q.refinement, false); e != nil {
		return e
	}
	for q.ownRemaining > 0 {
		if e := m.config.Integer.CleanupRawNumeral(q.values[q.ownRemaining-1].BorrowNumerator()); e != nil {
			return e
		}
		q.ownRemaining--
	}
	if e := q.binary.CleanupRefineInvocation(&q.refinement, true); e != nil {
		return e
	}
	q.active = false
	return nil
}
func (q *upolyIntervalBoundsScope) eval(v UPolynomialView, a *unumeral.BinaryRational) (int, error) {
	return q.owner.evalSignBinaryWithNegScopeAndReceipt(v, a, nil, &q.evaluation)
}

type upolyBoundTraceStream struct{ factorizationTraceStream }

func (s upolyBoundTraceStream) AppendUint32(v uint32) error {
	return linearWriteText(s.writer, strconv.FormatUint(uint64(v), 10))
}
func (q *upolyIntervalBoundsScope) trace(line int, a *unumeral.Rational) error {
	m := q.owner
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("convert_bug") {
		return nil
	}
	if !q.active || !q.executing || q.constructed != 2 {
		return ErrUPolynomialOwner
	}
	var computation error
	_ = tags.EmitConditional("convert_bug", "convert_q2bq_interval", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		out := upolyBoundTraceStream{factorizationTraceStream{factorizationTraceWriter{raw}}}
		if line == 2830 {
			_ = out.AppendCString("found c: ")
			if e := q.binary.DisplaySourceTracked(out, &q.values[0], &q.insertions[0]); e != nil {
				computation = e
				return e
			}
		} else {
			_ = out.AppendCString("a: ")
			// mpzzp::display delegates directly to integer insertion: no to_string
			// normalized assertion is added to either rational numeral.
			if e := m.config.Integer.InsertSourceIntegerTracked(out, a.BorrowNumerator(), &q.insertions[0], nil); e != nil {
				computation = e
				return e
			}
			_ = out.AppendCString("/")
			if e := m.config.Integer.InsertSourceIntegerTracked(out, a.BorrowDenominator(), &q.insertions[1], nil); e != nil {
				computation = e
				return e
			}
			_ = out.AppendCString("\nlower: ")
			if e := q.binary.DisplaySourceTracked(out, &q.values[0], &q.insertions[2]); e != nil {
				computation = e
				return e
			}
			_ = out.AppendCString(", upper: ")
			if e := q.binary.DisplaySourceTracked(out, &q.values[1], &q.insertions[3]); e != nil {
				computation = e
				return e
			}
		}
		_ = out.AppendCString("\n")
		return nil
	})
	return computation
}
