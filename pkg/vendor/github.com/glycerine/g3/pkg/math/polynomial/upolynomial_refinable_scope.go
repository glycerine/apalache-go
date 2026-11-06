package polynomial

// Source: upolynomial.cpp:2599-2744; mpbq.cpp:384-392.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

type upolyRefinableInvocationReceipt struct {
	owner      *UPolynomialManager
	invocation uint64
}
type upolyRefinableScope struct {
	parentSourceExecuting                      bool
	owner                                      *UPolynomialManager
	binary                                     *unumeral.BinaryRationalManager
	invocation                                 uint64
	active, executing, sourceExecuting, owning bool
	constructed, ownRemaining                  uint8
	values                                     [4]unumeral.BinaryRational
	insertions                                 [6]unumeral.NumeralInsertionReceipt
	evaluation                                 upolyBinaryEvaluationInvocationReceipt
	copy                                       unumeral.IntegerCopyAllocationReceipt
	roots                                      unumeral.NumericAllocationRoots
	hooks                                      unumeral.NumericInvocationHooks
	borrow                                     upolyFactorTraceBorrow
	binaryBinding                              unumeral.NumericCallerBinding
	bodyCause, sourceCause                     error
}

func (q *upolyRefinableScope) valid() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.refinable
}
func (q *upolyRefinableScope) NumericBound(executing bool) bool {
	return q.valid() && !q.owner.closed && q.active && (!executing || q.executing && !q.sourceExecuting && !q.owning && q.owner.callbackDepth == 0 && !q.owner.config.Integer.RuntimeOwner().FailureTransportActive() && !q.owner.modular.BorrowTraceTags().OutputCallbackActive())
}
func (q *upolyRefinableScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !q.NumericBound(true) || z != q.owner.config.Integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &q.roots
}
func (q *upolyRefinableScope) NumericExecuting() bool {
	return q.valid() && (q.executing || q.sourceExecuting || q.owning)
}
func (q *upolyRefinableScope) NumericBinaryExecuting(b *unumeral.BinaryRationalManager) bool {
	return q.NumericExecuting() && q.binary == b
}
func (m *UPolynomialManager) beginRefinable(b *unumeral.BinaryRationalManager, r *upolyRefinableInvocationReceipt) (*upolyRefinableScope, error) {
	q := &m.refinable
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	*q = upolyRefinableScope{owner: m, binary: b, invocation: q.invocation + 1, active: true, executing: true}
	q.hooks = unumeral.NumericInvocationHooks{Generation: q.invocation, Policy: q}
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
		*r = upolyRefinableInvocationReceipt{m, q.invocation}
	}
	return q, nil
}
func (q *upolyRefinableScope) construct() *unumeral.BinaryRational {
	i := q.constructed
	q.values[i] = unumeral.BinaryRational{}
	q.constructed++
	return &q.values[i]
}
func (q *upolyRefinableScope) eval(v UPolynomialView, a *unumeral.BinaryRational) (int, error) {
	return q.owner.evalSignBinaryWithNegScopeAndReceipt(v, a, nil, &q.evaluation)
}
func (q *upolyRefinableScope) pending() bool {
	if !q.active {
		return false
	}
	m := q.owner
	if q.constructed != 0 || unumeral.NumericAllocationPending(&q.roots) || m.config.Integer.MatchesCopyAllocationInvocation(&q.copy) {
		return true
	}
	if m.matchesBinaryEvaluationReceipt(&q.evaluation) && m.binaryEvaluation.active {
		return true
	}
	for i := range q.insertions {
		if q.insertions[i].Active() {
			return true
		}
	}
	return false
}
func (q *upolyRefinableScope) childSourceCause() error {
	for i := range q.insertions {
		if q.insertions[i].Active() {
			if e := q.insertions[i].SourceCause(); e != nil {
				return e
			}
		}
	}
	if q.owner.matchesBinaryEvaluationReceipt(&q.evaluation) {
		return q.owner.binaryEvaluation.sourceCause
	}
	return nil
}
func (q *upolyRefinableScope) finish(body error) error {
	q.executing = false
	defer q.borrow.unbind()
	defer func() {
		if e := q.binaryBinding.Unbind(); e != nil {
			panic(e)
		}
	}()
	q.bodyCause = body
	q.sourceCause = q.childSourceCause()
	if q.sourceCause == nil {
		q.sourceExecuting = true
		for q.constructed > 0 {
			if _, e := upolynomialSourceNumeralDel(q.owner.config.Integer, q.values[q.constructed-1].BorrowNumerator()); e != nil {
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
func (m *UPolynomialManager) refinableReceiptPending(r *upolyRefinableInvocationReceipt) bool {
	return r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.refinable.invocation && m.refinable.active
}
func (m *UPolynomialManager) cleanupRefinableReceipt(r *upolyRefinableInvocationReceipt) error {
	if !m.refinableReceiptPending(r) {
		return nil
	}
	return m.CleanupRefinable()
}
func (m *UPolynomialManager) CleanupRefinable() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.refinable
	if !q.active {
		return nil
	}
	if !q.valid() {
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
	if e := q.roots.Cleanup(m.config.Integer); e != nil {
		return e
	}
	if e := m.config.Integer.CleanupCopyAllocationInvocation(&q.copy); e != nil {
		return e
	}
	for q.ownRemaining > 0 {
		if e := m.config.Integer.CleanupRawNumeral(q.values[q.ownRemaining-1].BorrowNumerator()); e != nil {
			return e
		}
		q.ownRemaining--
	}
	q.active = false
	return nil
}
func (q *upolyRefinableScope) traceSigns(a, b int) error {
	tags := q.owner.modular.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	if !q.NumericBound(true) {
		return ErrUPolynomialOwner
	}
	_ = tags.EmitConditional("upolynomial", "isolating2refinable", "src/math/polynomial/upolynomial.cpp", 2602, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		_ = linearWriteText(out, "sign_a: ")
		_ = linearWriteText(out, strconv.Itoa(a))
		_ = linearWriteText(out, ", sign_b: ")
		_ = linearWriteText(out, strconv.Itoa(b))
		_ = linearWriteText(out, "\n")
		return nil
	})
	return nil
}
func (q *upolyRefinableScope) traceInterval(line int, values [6]*unumeral.BinaryRational) error {
	tags := q.owner.modular.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	if !q.NumericBound(true) {
		return ErrUPolynomialOwner
	}
	labels := [6]string{"CASE 2, a: ", ", b: ", ", new_a: "}
	count := 3
	if line == 2640 {
		labels[0] = "CASE 3, a: "
		labels[2] = ", new_b: "
	}
	if line == 2693 {
		labels = [6]string{"CASE 4\na1: ", ", b1: ", ", new_a1: ", "\na2: ", ", b2: ", ", new_b2: "}
		count = 6
	}
	var computation error
	_ = tags.EmitConditional("upolynomial", "isolating2refinable", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i := 0; i < count; i++ {
			_ = linearWriteText(out, labels[i])
			text, e := q.binary.SourceStringTracked(values[i], &q.insertions[i])
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, text)
		}
		_ = linearWriteText(out, "\n")
		return nil
	})
	return computation
}

func (m *UPolynomialManager) refinableReceiptSourceCause(r *upolyRefinableInvocationReceipt) error {
	if m.refinableReceiptPending(r) {
		return m.refinable.sourceCause
	}
	return nil
}
func (q *upolyRefinableScope) sourceReadAllowed() bool {
	return q.valid() && (q.sourceExecuting || q.parentSourceExecuting) && q.owner.callbackDepth == 0 && !q.owner.config.Integer.RuntimeOwner().FailureTransportActive() && !q.owner.modular.BorrowTraceTags().OutputCallbackActive()
}
