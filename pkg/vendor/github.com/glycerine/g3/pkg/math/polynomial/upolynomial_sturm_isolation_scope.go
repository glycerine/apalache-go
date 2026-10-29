package polynomial

// Source: upolynomial.cpp:2392-2483; actual internal isolation stack locals.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolySturmIsolationScope struct {
	traceDisplays                     [2]upolyDisplayInvocationReceipt
	traceInsertions                   [3]unumeral.NumeralInsertionReceipt
	normalization                     upolySturmNormalizeScope
	hooks                             unumeral.NumericInvocationHooks
	numeric                           unumeral.NumericAllocationRoots
	raw                               []unumeral.IntegerRawInvocationReceipt
	knuth                             upolyKnuthInvocationReceipt
	prepareNormalize                  unumeral.IntegerNormalizeInvocationReceipt
	prepared                          upolyVectorState
	preparedLive                      bool
	preparedSource, preparedOwn       uint32
	endpoint                          upolyBinaryEvaluationInvocationReceipt
	copy                              unumeral.IntegerCopyAllocationReceipt
	owner                             *UPolynomialManager
	invocation                        uint64
	target                            UPolynomialView
	binary                            *unumeral.BinaryRationalManager
	sequence                          UPolynomialSequence // source scoped sequence; no public owner/header allocation
	mid, lower, upper                 unumeral.BinaryRational
	stack                             upolySSStack
	live                              [5]bool // sequence, mid, lower, upper, frame stack
	active, executing, finishing      bool
	sourceStage, ownStage             uint8
	sourceStack, ownStack             uint64
	sourceCoefficient, ownCoefficient uint32
	bodyCause, sourceStop, childStop  error
	integerBinding, naturalBinding    unumeral.NumericCallerBinding
}

func (q *upolySturmIsolationScope) valid() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.sturmIsolation
}
func (q *upolySturmIsolationScope) NumericExecuting() bool {
	return q.valid() && q.active && (q.executing || q.finishing)
}
func (q *upolySturmIsolationScope) bind() error {
	z := q.owner.config.Integer
	if e := z.BindCaller(&q.integerBinding, q, 18); e != nil {
		return e
	}
	if e := z.Naturals().BindCaller(&q.naturalBinding, q, 18); e != nil {
		return errors.Join(e, q.integerBinding.Unbind())
	}
	return nil
}
func (q *upolySturmIsolationScope) unbind() error {
	return errors.Join(q.naturalBinding.Unbind(), q.integerBinding.Unbind())
}
func (m *UPolynomialManager) beginSturmIsolation(v UPolynomialView, b *unumeral.BinaryRationalManager) (*upolySturmIsolationScope, error) {
	q := &m.sturmIsolation
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	sequenceInvocation := q.sequence.sturm.invocation
	*q = upolySturmIsolationScope{owner: m, invocation: q.invocation + 1, target: v, binary: b, active: true, executing: true}
	// The embedded source object's address is reused; retired numeric tuples
	// must retain a distinct generation across enclosing isolation invocations.
	q.sequence.sturm.invocation = sequenceInvocation
	q.hooks.Policy = q
	q.hooks.Generation = q.invocation
	if e := q.bind(); e != nil {
		q.active = false
		q.executing = false
		return nil, e
	}
	return q, nil
}
func (q *upolySturmIsolationScope) pending() bool {
	for _, live := range q.live {
		if live {
			return true
		}
	}
	return q.tracePending() || q.normalizePending() || q.numericPending() || q.knuthPending() || q.owner.modular.MatchesPNormalizeInvocation(&q.prepareNormalize) || q.preparedLive || q.sequence.sturm.active || q.sequence.evaluationPending() || q.owner.matchesBinaryEvaluationReceipt(&q.endpoint) && q.owner.binaryEvaluation.active || q.owner.config.Integer.MatchesCopyAllocationInvocation(&q.copy)
}
func (q *upolySturmIsolationScope) sourceChild() error {
	if e := q.traceSourceCause(); e != nil {
		return e
	}
	if q.normalization.sourceStop != nil {
		return q.normalization.sourceStop
	}
	if e := q.numericSourceCause(); e != nil {
		return e
	}
	if q.knuth == q.owner.knuthReceipt() && q.owner.knuth.sourceCause != nil {
		return q.owner.knuth.sourceCause
	}
	s := &q.sequence
	if s.sturm.sourceStop != nil || s.sturm.childStop != nil {
		return errors.Join(s.sturm.sourceStop, s.sturm.childStop)
	}
	if q.owner.matchesBinaryEvaluationReceipt(&q.endpoint) && q.owner.binaryEvaluation.sourceCause != nil {
		return q.owner.binaryEvaluation.sourceCause
	}
	return s.evaluationSourceStop
}
func (q *upolySturmIsolationScope) stackNumeral(i uint64) *unumeral.Int {
	f := &q.stack.values[i/2]
	if i%2 == 0 {
		return &f.lower.Num
	}
	return &f.upper.Num
}
func (q *upolySturmIsolationScope) scalar(stage uint8) *unumeral.Int {
	switch stage {
	case 1:
		return &q.upper.Num
	case 2:
		return &q.lower.Num
	default:
		return &q.mid.Num
	}
}

// Literal source destruction has independent cursors. A stopped child or
// destructor freezes every older declaration; owning cleanup never edits them.
func (q *upolySturmIsolationScope) finishSource(cause error) error {
	if q.bodyCause == nil {
		q.bodyCause = cause
	}
	if q.childStop == nil {
		q.childStop = q.sourceChild()
	}
	if q.sourceStop != nil || q.childStop != nil {
		return errors.Join(cause, q.sourceStop, q.childStop)
	}
	q.finishing = true
	defer func() { q.finishing = false }()
	m, z := q.owner, q.owner.config.Integer
	for q.sourceStage < 5 {
		stage := q.sourceStage
		if q.live[4-stage] {
			var e error
			switch stage {
			case 0:
				for q.sourceStack < uint64(len(q.stack.values))*2 {
					if e = unumeral.SourceIntegerDel(z, q.stackNumeral(q.sourceStack)); e != nil {
						break
					}
					q.sourceStack++
				}
				if e == nil {
					q.stack.values = nil
				}
			case 1, 2, 3:
				e = unumeral.SourceIntegerDel(z, q.scalar(stage))
			case 4:
				state := &q.sequence.state.coefficients
				values := m.values(state)
				for q.sourceCoefficient < state.size {
					if e = unumeral.SourceIntegerDel(z, &values[q.sourceCoefficient]); e != nil {
						break
					}
					q.sourceCoefficient++
				}
				if e == nil {
					state.size = 0
					m.freeBacking(state)
					q.sequence.state.begins = nil
					q.sequence.state.sizes = nil
				}
			}
			if e != nil {
				q.sourceStop = e
				return errors.Join(cause, e)
			}
			q.live[4-stage] = false
		}
		q.sourceStage++
	}
	if q.preparedLive {
		values := m.values(&q.prepared)
		for q.preparedSource < q.prepared.size {
			if e := unumeral.SourceIntegerDel(z, &values[q.preparedSource]); e != nil {
				q.sourceStop = e
				return errors.Join(cause, e)
			}
			q.preparedSource++
		}
		q.prepared.size = 0
		m.freeBacking(&q.prepared)
		q.preparedLive = false
	}
	return cause
}
func (q *upolySturmIsolationScope) finish(cause error) (result error) {
	result = q.finishSource(cause)
	q.executing = false
	if !q.pending() {
		q.active = false
	}
	return errors.Join(result, q.unbind())
}
func (m *UPolynomialManager) sturmIsolationExecuting() bool {
	return m != nil && m.self == m && m.sturmIsolation.NumericExecuting()
}
func (m *UPolynomialManager) sturmIsolationSourceCause() error {
	if m.sturmIsolation.active {
		q := &m.sturmIsolation
		if q.sourceStop != nil || q.childStop != nil {
			return errors.Join(q.sourceStop, q.childStop)
		}
		return ErrUPolynomialCleanupPending
	}
	return nil
}

// CleanupSturmIsolation acknowledges only this manager's accepted isolation
// invocation and its exact children. Literal source cursors remain unchanged.
func (m *UPolynomialManager) CleanupSturmIsolation() (cause error) {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.sturmIsolation
	if !q.active {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	defer func() { q.finishing = false }()
	if e := q.bind(); e != nil {
		return e
	}
	defer func() { cause = errors.Join(cause, q.unbind()) }()
	z := m.config.Integer
	if e := q.cleanupTrace(false); e != nil {
		return e
	}
	if e := q.cleanupNumeric(false); e != nil {
		return e
	}
	if e := q.cleanupNormalization(); e != nil {
		return e
	}
	if e := m.cleanupBinaryEvaluationWithReceipt(&q.endpoint, false); e != nil {
		return e
	}
	if e := m.cleanupBinaryEvaluationWithReceipt(&q.sequence.evaluation, false); e != nil {
		return e
	}
	if e := m.cleanupKnuthWithReceipt(&q.knuth, false); e != nil {
		return e
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.prepareNormalize, true); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&q.copy); e != nil {
		return e
	}
	// Only the constructor belonging to the actual internal sequence is owned.
	if q.sequence.self == &q.sequence {
		if e := q.sequence.CleanupSturm(); e != nil {
			return e
		}
	}
	for q.ownStage < 5 {
		stage := q.ownStage
		if q.live[4-stage] {
			switch stage {
			case 0:
				for q.ownStack < uint64(len(q.stack.values))*2 {
					if e := z.CleanupRawNumeral(q.stackNumeral(q.ownStack)); e != nil {
						return e
					}
					q.ownStack++
				}
				q.stack.values = nil
			case 1, 2, 3:
				if e := z.CleanupRawNumeral(q.scalar(stage)); e != nil {
					return e
				}
			case 4:
				state := &q.sequence.state.coefficients
				values := m.values(state)
				for q.ownCoefficient < state.size {
					if e := z.CleanupRawNumeral(&values[q.ownCoefficient]); e != nil {
						return e
					}
					q.ownCoefficient++
				}
				state.size = 0
				m.freeBacking(state)
				q.sequence.state.begins = nil
				q.sequence.state.sizes = nil
			}
			q.live[4-stage] = false
		}
		q.ownStage++
	}
	if q.preparedLive {
		values := m.values(&q.prepared)
		for q.preparedOwn < q.prepared.size {
			if e := z.CleanupRawNumeral(&values[q.preparedOwn]); e != nil {
				return e
			}
			q.preparedOwn++
		}
		q.prepared.size = 0
		m.freeBacking(&q.prepared)
		q.preparedLive = false
	}
	if e := m.cleanupBinaryEvaluationWithReceipt(&q.sequence.evaluation, true); e != nil {
		return e
	}
	if e := m.cleanupBinaryEvaluationWithReceipt(&q.endpoint, true); e != nil {
		return e
	}
	if e := m.cleanupKnuthWithReceipt(&q.knuth, true); e != nil {
		return e
	}
	if e := q.cleanupTrace(true); e != nil {
		return e
	}
	if e := q.cleanupNumeric(true); e != nil {
		return e
	}
	q.active = false
	return nil
}

func (q *upolySturmIsolationScope) copyBound(dst, src *unumeral.BinaryRational) error {
	if !q.entryValid() {
		return ErrUPolynomialOwner
	}
	return q.binary.CopyWithAllocationReceipt(dst, src, &q.copy)
}

func (q *upolySturmIsolationScope) constructCore() {
	m := q.owner
	// Actual declarations: sequence, mid, curr_lower, curr_upper. Native owner
	// identity represents the source stack object without a NewSequence reserve.
	q.sequence.manager = m
	q.sequence.self = &q.sequence
	q.live = [5]bool{true, true, true, true, false}
}

func (q *upolySturmIsolationScope) knuthPending() bool {
	return q.knuth == q.owner.knuthReceipt() && q.owner.knuth.active
}
func (q *upolySturmIsolationScope) setPrepared(v UPolynomialView) error {
	if !q.entryValid() {
		return ErrUPolynomialOwner
	}
	m := q.owner
	if e := m.reserve(&q.prepared, v.count); e != nil {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		if e = m.modular.CopyFromOwnerTracked(&m.values(&q.prepared)[i], m.config.Integer, a, &q.copy, &q.prepareNormalize); e != nil {
			return e
		}
	}
	return m.setSize(&q.prepared, v.count)
}

// Close of this exact borrowed binary manager must not destroy shared scratch
// while a checkpoint/failure callback suspends the actual isolation invocation.
func (q *upolySturmIsolationScope) NumericBinaryExecuting(b *unumeral.BinaryRationalManager) bool {
	return q.NumericExecuting() && q.binary == b
}

func (q *upolySturmIsolationScope) entryValid() bool {
	if !q.valid() || !q.active || !q.executing || q.finishing || q.owner.closed || q.owner.callbackDepth != 0 {
		return false
	}
	z := q.owner.config.Integer
	return !z.RuntimeOwner().FailureTransportActive() && z.CallerBindings() == &q.integerBinding && z.Naturals().CallerBindings() == &q.naturalBinding
}

// The outer overload's actual scoped p1 declaration precedes its first SET.
func (q *upolySturmIsolationScope) constructPrepared(v UPolynomialView) error {
	if !q.entryValid() {
		return ErrUPolynomialOwner
	}
	if q.preparedLive {
		return ErrUPolynomialCleanupPending
	}
	q.preparedLive = true
	return q.setPrepared(v)
}

// The source frame stack is declared only after sturm_seq returns successfully.
func (q *upolySturmIsolationScope) constructStack() error {
	if !q.entryValid() {
		return ErrUPolynomialOwner
	}
	if q.live[4] {
		return ErrUPolynomialCleanupPending
	}
	q.live[4] = true
	return nil
}
