package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1579-1647,2937-2939.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrAlgebraicDivisionByZero error = newAlgebraicException("division by zero")

// The two source scoped rationals survive a refused destruction in the owner.
type algebraicInverseScope struct {
	refineEvaluation              upolyBinaryEvaluationInvocationReceipt
	refineCopy                    unumeral.IntegerCopyAllocationReceipt
	conversions                   [2]algebraicConversionScope
	owner                         *AlgebraicRootCore
	numericHooks                  unumeral.NumericInvocationHooks
	numericRoots                  unumeral.NumericAllocationRoots
	traceDisplayPending           bool
	traceDisplay                  algebraicRootDisplayInvocationReceipt
	sourceCursor, sourceRemaining uint8
	convertEntry                  upolyConvertEntryReceipt
	polynomialCaller              *polynomialAlgebraicEvaluationScope
	sourceStop                    error
	values                        [2]unumeral.Rational // inv_lower, inv_upper
	remaining                     uint8
	executing                     bool
}

func (q *algebraicInverseScope) NumericBound(executing bool) bool {
	m := q.owner
	return m != nil && m.self == m && !m.closed && q == &m.inversion && (!executing || q.executing)
}

func (q *algebraicInverseScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !q.NumericBound(true) || q.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &q.numericRoots
}

func (q *algebraicInverseScope) numericPending(m *AlgebraicRootCore) bool {
	return q.owner == m && unumeral.NumericAllocationPending(&q.numericRoots)
}

func (q *algebraicInverseScope) numericContext() unumeral.NumericAllocationContext {
	return unumeral.NumericAllocationContext{}.WithCaller(&q.numericHooks)
}

func (m *AlgebraicRootCore) inverseRefinementEvaluationPending() bool {
	return m != nil && m.self == m && m.config.Polynomials.matchesBinaryEvaluationReceipt(&m.inversion.refineEvaluation) && m.config.Polynomials.binaryEvaluation.active
}

func (m *AlgebraicRootCore) inverseRefinementCopyPending() bool {
	return m != nil && m.self == m && m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&m.inversion.refineCopy)
}

// Literal reverse scoped_mpq destruction, NUM before DEN. Owning cleanup
// below may consume the suffix without replaying or advancing this cursor.
func (m *AlgebraicRootCore) finishInverseSource() error {
	q := &m.inversion
	if q.sourceStop != nil {
		return q.sourceStop
	}
	if e := m.rootTraceSourceCause(); e != nil {
		return e
	}
	for i := len(q.conversions) - 1; i >= 0; i-- {
		if q.conversions[i].stop != nil {
			return q.conversions[i].stop
		}
	}
	q.executing = true
	defer func() { q.executing = false }()
	for q.remaining != 0 {
		value := &q.values[q.remaining-1]
		field := value.BorrowNumerator()
		if q.sourceCursor%2 != 0 {
			field = value.BorrowDenominator()
		}
		if e := unumeral.SourceIntegerDel(m.config.Rationals.Integers(), field); e != nil {
			q.sourceRemaining = q.remaining
			return e
		}
		q.sourceCursor++
		if q.sourceCursor%2 == 0 {
			q.remaining--
		}
	}
	q.sourceRemaining = q.remaining
	return nil
}
func (m *AlgebraicRootCore) cleanupInverse() error {
	q := &m.inversion
	if q.executing {
		return ErrAlgebraicPending
	}
	q.executing = true
	defer func() { q.executing = false }()
	if e := m.config.Polynomials.cleanupBinaryEvaluationWithReceipt(&q.refineEvaluation, false); e != nil {
		return e
	}
	if e := m.config.Polynomials.cleanupBinaryEvaluationWithReceipt(&q.refineEvaluation, true); e != nil {
		return e
	}
	if e := m.config.Rationals.Integers().CleanupCopyAllocationInvocation(&q.refineCopy); e != nil {
		return e
	}
	if q.traceDisplayPending {
		if e := m.cleanupRootDisplayInvocation(&q.traceDisplay); e != nil {
			return e
		}
		q.traceDisplayPending = false
	}
	for i := len(q.conversions) - 1; i >= 0; i-- {
		if e := q.conversions[i].cleanup(); e != nil {
			return e
		}
	}
	if e := q.numericRoots.Cleanup(m.config.Rationals.Integers()); e != nil {
		return e
	}
	for q.remaining != 0 {
		value := &q.values[q.remaining-1]
		for _, field := range [2]*unumeral.Int{value.BorrowNumerator(), value.BorrowDenominator()} {
			if e := m.config.Rationals.Integers().CleanupRawNumeral(field); e != nil {
				return e
			}
		}
		q.remaining--
	}
	return nil
}

// refineNonzeroBounds follows the source halving search. General refinement
// would change the nonzero endpoint as well and would not preserve this path.
func (m *AlgebraicRootCore) refineNonzeroBounds(a *Algebraic) error {
	l, e := m.location(a)
	if e != nil || l == nil || l.kind == 0 {
		return e
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	q := &m.inversion
	q.refineEvaluation = upolyBinaryEvaluationInvocationReceipt{}
	q.executing = true
	defer func() { q.executing = false }()
	b := m.config.Binary
	lower, upper := r.interval.BorrowLower(), r.interval.BorrowUpper()
	lz, e := b.IsZero(lower)
	if e != nil {
		return e
	}
	if !lz {
		uz, e := b.IsZero(upper)
		if e != nil || !uz {
			return e
		}
	}
	sign := 1
	if r.flags&2 != 0 {
		sign = -1
	}
	if e = algebraicIsolationAssert(sign != 0, 1591, "sign_l != 0"); e != nil {
		return e
	}
	bound, other := lower, upper
	if !lz {
		d, enabled, e := algebraicAssertionSwitch()
		if e != nil {
			return e
		}
		if enabled {
			zero, e := b.IsZero(upper)
			if e != nil {
				return e
			}
			if !zero {
				if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1614, "bqm().is_zero(upper)"); e != nil {
					return e
				}
			}
		}
		bound, other = upper, lower
		sign = -sign
	}
	e = b.CopyWithAllocationReceipt(bound, other, &q.refineCopy)
	if e = m.save(l, &r, e); e != nil {
		return e
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	for {
		e = b.Div2(bound)
		if e = m.save(l, &r, e); e != nil {
			return e
		}
		next, e := m.config.Polynomials.evalSignBinaryWithNegScopeAndReceipt(v, bound, nil, &q.refineEvaluation)
		if e != nil {
			return e
		}
		if next == 0 {
			if e = m.beginQ(); e != nil {
				return e
			}
			e = m.refinementConversion.run(m, bound, &m.localQ)
			if e == nil {
				q.executing = false
				e = m.SetTake(a, &m.localQ)
				q.executing = true
			}
			if m.refinementConversion.stop != nil {
				return e
			}
			return m.finishQ(e)
		}
		if next == sign {
			return nil
		}
	}
}

// Inv replaces a nonzero value in place. An algebraic root keeps its cell and
// minimality flag while reversing its polynomial and reciprocating its bounds.
func (m *AlgebraicRootCore) Inv(a *Algebraic) (cause error) {
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if l == nil {
		if e := diagnostic.BorrowProcessDebugContext().UnreachableSource("src/math/polynomial/algebraic_numbers.cpp", 1622); e != nil {
			return e
		}
		return ErrAlgebraicDivisionByZero
	}
	if m.inversion.remaining != 0 || m.inversion.numericPending(m) || m.localActive || m.negation.active {
		return ErrAlgebraicPending
	}
	for i := range m.inversion.conversions {
		if m.inversion.conversions[i].pending(m) {
			return ErrAlgebraicPending
		}
	}
	if m.inversion.numericHooks.Generation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	if e = m.refineNonzeroBounds(a); e != nil {
		return e
	}
	l, e = m.location(a)
	if e != nil {
		return e
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	q := &m.inversion
	q.owner = m
	q.numericRoots = unumeral.NumericAllocationRoots{}
	q.numericHooks = unumeral.NumericInvocationHooks{Generation: q.numericHooks.Generation + 1, Policy: q}
	q.polynomialCaller = m.polynomialCaller
	q.sourceStop = nil
	q.traceDisplayPending = false
	q.sourceCursor, q.sourceRemaining = 0, 0
	q.executing = true
	defer func() {
		q.executing = false
		cleanup := m.rootTraceSourceCause()
		if cleanup == nil {
			cleanup = m.config.Polynomials.convertEntryReceiptSourceCause(&q.convertEntry)
		}
		if cleanup == nil {
			cleanup = m.finishInverseSource()
		}
		if cleanup != nil {
			q.sourceStop = cleanup
		}
		cause = errors.Join(cause, cleanup)
	}()
	if l.kind == 0 {
		e = m.config.Rationals.WithAllocationContext(q.numericContext()).InvInPlace(&r.value)
		return m.save(l, &r, e)
	}
	p := m.config.Polynomials
	if e = m.traceInverseValue(a, false); e != nil {
		return e
	}
	// Source display_root may publish the cached root index before cell_a is
	// read. Keep that actual cell mutation in subsequent interval saves.
	r, e = m.load(l)
	if e != nil {
		return e
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	if e = p.ReciprocalViewInPlace(v); e != nil {
		return e
	}
	q.values = [2]unumeral.Rational{unumeral.NewRational(), unumeral.NewRational()}
	q.remaining = 2
	q.sourceRemaining = 2
	qm, b := m.config.Rationals, m.config.Binary
	if e = q.conversions[0].run(m, r.interval.BorrowLower(), &q.values[0]); e != nil {
		return e
	}
	if e = q.conversions[1].run(m, r.interval.BorrowUpper(), &q.values[1]); e != nil {
		return e
	}
	qops := qm.WithAllocationContext(q.numericContext())
	if e = qops.InvInPlace(&q.values[0]); e != nil {
		return e
	}
	if e = qops.InvInPlace(&q.values[1]); e != nil {
		return e
	}
	if e = qm.Swap(&q.values[0], &q.values[1]); e != nil {
		return e
	}
	if e = m.traceInverseBounds(); e != nil {
		return e
	}
	_, e = p.convertRationalToBinaryIntervalWithEntryReceipt(v, qm, &q.values[0], &q.values[1], b, r.interval.BorrowLower(), r.interval.BorrowUpper(), &q.convertEntry)
	if e = m.save(l, &r, e); e != nil {
		return e
	}
	return m.traceInverseValue(a, true)
}
