package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:383-388,1565-1577;
// src/math/polynomial/upolynomial.cpp:1734-1764.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Neg itself declares no NUM. Its reached binary Horner child declares r then
// ak; these fields are their actual values, charged in the containing core.
type algebraicNegScope struct {
	owner                                            *AlgebraicRootCore
	target                                           Algebraic
	invocation                                       uint64
	active, executing, sourceExecuting, ownExecuting bool
	falseAcknowledged                                bool
	numeric                                          polynomialNumericRoots
	normalize                                        unumeral.IntegerNormalizeInvocationReceipt
	eval                                             algebraicNegBinaryScope
	evalHistory                                      [2]algebraicNegBinaryHistory
	bodyCause, sourceCause, cleanupCause             error
	phase                                            uint8
	historyRecorded                                  bool
	historyIndex                                     int
}
type algebraicNegBinaryScope struct {
	invocation                                  uint64
	values                                      [2]unumeral.Int
	numeric                                     polynomialNumericRoots
	normalize                                   unumeral.IntegerNormalizeInvocationReceipt
	addMul                                      unumeral.IntegerAddMulInvocationReceipt
	constructed, sourceRemaining, ownRemaining  uint8
	active, executing, bodyReturned, ownStarted bool
	bodyCause, sourceCause                      error
}
type algebraicNegInvocationReceipt struct {
	owner      *AlgebraicRootCore
	frame      *algebraicNegScope
	invocation uint64
}

func (m *AlgebraicRootCore) negationExecuting() bool {
	if m == nil {
		return false
	}
	q := &m.negation
	return m != nil && m.self == m && q.owner == m && q.invocation != 0 && q.active && (q.executing || q.sourceExecuting || q.ownExecuting)
}
func (m *AlgebraicRootCore) matchesNegation(r *algebraicNegInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.frame == &m.negation && r.invocation != 0 && r.frame.owner == m && r.invocation == r.frame.invocation && r.frame.active
}
func (c integerAllocationInvocationContext) algebraicNegFrame(z *unumeral.IntegerManager) *algebraicNegScope {
	q := c.algebraicNeg
	if z == nil || !z.ValidOwner() || q == nil || q.owner == nil || q.owner.self != q.owner || q != &q.owner.negation || q.owner.config.Rationals == nil || q.owner.config.Rationals.Integers() != z || algebraicNegCaller(z) != q.owner || !q.active || !q.executing || c.algebraicNegInvocation == 0 || c.algebraicNegInvocation != q.invocation || q.owner.config.Polynomials == nil || q.owner.config.Polynomials.self != q.owner.config.Polynomials || q.owner.config.Polynomials.callbackDepth != 0 || q.owner.config.Polynomials.algebraicNegCaller != q.owner {
		return nil
	}
	if c.algebraicNegEval && (!q.eval.active || !q.eval.executing || q.eval.constructed != 2 || c.algebraicNegEvalInvocation == 0 || c.algebraicNegEvalInvocation != q.eval.invocation) {
		return nil
	}
	return q
}
func (q *algebraicNegScope) context(eval bool) integerAllocationInvocationContext {
	return integerAllocationInvocationContext{algebraicNeg: q, algebraicNegInvocation: q.invocation, algebraicNegEval: eval, algebraicNegEvalInvocation: q.eval.invocation}
}
func (q *algebraicNegScope) pending() bool {
	z := q.owner.config.Rationals.Integers()
	mm := q.owner.config.Polynomials.modular
	return polynomialNegNumericPending(&q.numeric) || polynomialNegNumericPending(&q.eval.numeric) || q.eval.sourceRemaining != 0 || mm.MatchesPNormalizeInvocation(&q.normalize) || mm.MatchesPNormalizeInvocation(&q.eval.normalize) || z.MatchesAddMulInvocation(&q.eval.addMul)
}
func (q *algebraicNegScope) childSourceCause() error {
	if q.sourceCause == nil {
		q.sourceCause = q.eval.sourceCause
	}
	return q.sourceCause
}
func (q *algebraicNegScope) finish(cause error) error {
	if q.bodyCause == nil {
		q.bodyCause = cause
	}
	q.executing = false
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false; q.updateHistory() }()
	q.childSourceCause()
	if !q.pending() {
		q.active = false
	}
	return errors.Join(q.bodyCause, q.sourceCause)
}

func (m *AlgebraicRootCore) cleanupNegation(r *algebraicNegInvocationReceipt, abort bool) (cause error) {
	if !m.matchesNegation(r) {
		return nil
	}
	q := r.frame
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrAlgebraicPending
	}
	if abort && m.negationTargetLive(q) {
		return ErrAlgebraicLive
	}
	q.ownExecuting = true
	zCaller, upCaller := algebraicNegCaller(m.config.Rationals.Integers()), m.config.Polynomials.algebraicNegCaller
	setAlgebraicNegCaller(m.config.Rationals.Integers(), m)
	m.config.Polynomials.algebraicNegCaller = m
	defer func() {
		setAlgebraicNegCaller(m.config.Rationals.Integers(), zCaller)
		m.config.Polynomials.algebraicNegCaller = upCaller
	}()
	defer func() {
		q.ownExecuting = false
		if q.cleanupCause == nil {
			q.cleanupCause = cause
		}
		q.updateHistory()
	}()
	z := m.config.Rationals.Integers()
	mm := m.config.Polynomials.modular
	if e := mm.CleanupPNormalizeInvocation(&q.eval.normalize, false); e != nil {
		return e
	}
	if e := z.CleanupAddMulInvocation(&q.eval.addMul, false); e != nil {
		return e
	}
	if !q.eval.ownStarted {
		q.eval.ownRemaining = q.eval.constructed
		q.eval.ownStarted = true
	}
	for q.eval.ownRemaining != 0 {
		if e := z.CleanupRawNumeral(&q.eval.values[q.eval.ownRemaining-1]); e != nil {
			return e
		}
		q.eval.ownRemaining--
	}
	if e := mm.CleanupPNormalizeInvocation(&q.normalize, false); e != nil {
		return e
	}
	q.falseAcknowledged = true
	if !abort {
		return nil
	}
	if e := mm.CleanupPNormalizeInvocation(&q.eval.normalize, true); e != nil {
		return e
	}
	if e := z.CleanupAddMulInvocation(&q.eval.addMul, true); e != nil {
		return e
	}
	if e := cleanupUPolynomialNumericRoots(z, &q.eval.numeric); e != nil {
		return e
	}
	if e := mm.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return e
	}
	if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
		return e
	}
	q.active = false
	return nil
}

// CleanupNegation explicitly acknowledges only the actual current call. The
// caller's algebraic value is never disposed by this child acknowledgment.
func (m *AlgebraicRootCore) CleanupNegation() error {
	if m != nil && m.self == m && (m.comparisonTraceRunning || m.rootTrace.running || m.univariateIsolationRunning) {
		return ErrAlgebraicPending
	}
	if m == nil || m.self != m {
		return ErrAlgebraicOwner
	}
	r := algebraicNegInvocationReceipt{m, &m.negation, m.negation.invocation}
	if e := m.cleanupNegation(&r, false); e != nil {
		return e
	}
	return m.cleanupNegation(&r, true)
}

func (m *AlgebraicRootCore) Neg(a *Algebraic) (cause error) {
	return m.negWithReceipt(a, nil)
}
func (m *AlgebraicRootCore) negWithReceipt(a *Algebraic, receipt *algebraicNegInvocationReceipt) (cause error) {
	l, e := m.location(a)
	if e != nil || l == nil {
		return e
	}
	q := &m.negation
	if q.active {
		return ErrAlgebraicPending
	}
	if q.invocation == math.MaxUint64 {
		return arena.ErrBounds
	}
	nonce := q.invocation + 1
	*q = algebraicNegScope{owner: m, target: *a, invocation: nonce, active: true, executing: true}
	if receipt != nil {
		*receipt = algebraicNegInvocationReceipt{owner: m, frame: q, invocation: nonce}
	}
	z := m.config.Rationals.Integers()
	up := m.config.Polynomials
	previousZ, previousUP := algebraicNegCaller(z), up.algebraicNegCaller
	setAlgebraicNegCaller(z, m)
	up.algebraicNegCaller = m
	defer func() {
		cause = q.finish(cause)
		setAlgebraicNegCaller(z, previousZ)
		up.algebraicNegCaller = previousUP
	}()
	r, e := m.load(l)
	if e != nil {
		return e
	}
	context := q.context(false)
	if l.kind == 0 {
		q.phase = 1
		e = m.config.Rationals.WithAllocationContext((context).utility(m.config.Rationals.Integers())).Neg(&r.value)
		return m.save(l, &r, e)
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	q.phase = 2
	if e = up.pMinusXWithSourceContext(v, context); e != nil {
		return e
	}
	q.phase = 3
	e = m.config.Intervals.WithAllocationContext((context).utility(m.config.Intervals.BorrowBounds().Integers())).Neg(&r.interval)
	if e = m.save(l, &r, e); e != nil {
		return e
	}
	q.phase = 4
	sign, e := up.evalSignBinaryWithNegScope(v, &(*r.interval.BorrowLower()), q)
	if e != nil {
		return e
	}
	if polynomialSourceDebug {
		if sign == 0 {
			return ErrUPolynomialAssertion
		}
		q.phase = 5
		upper, e := up.evalSignBinaryWithNegScope(v, &(*r.interval.BorrowUpper()), q)
		if e != nil {
			return e
		}
		if upper != -sign {
			return ErrUPolynomialAssertion
		}
	}
	r.flags &^= 2
	if sign < 0 {
		r.flags |= 2
	}
	q.phase = 6
	return m.store(l, &r)
}

func (q *algebraicNegScope) finishBinarySource(body error) error {
	e := &q.eval
	if !e.bodyReturned || e.executing {
		return ErrAlgebraicPending
	}
	defer func() {
		if e.invocation >= 1 && e.invocation <= 2 {
			q.evalHistory[e.invocation-1] = q.binaryDiagnostic()
		}
	}()
	z := q.owner.config.Rationals.Integers()
	mm := q.owner.config.Polynomials.modular
	if e.sourceCause == nil {
		e.sourceCause = z.AddMulInvocationSourceCause(&e.addMul)
	}
	if e.sourceCause != nil {
		return errors.Join(body, e.sourceCause)
	}
	for e.sourceRemaining != 0 {
		if _, err := upolynomialSourceNumeralDel(z, &e.values[e.sourceRemaining-1]); err != nil {
			e.sourceCause = err
			return errors.Join(body, err)
		}
		e.sourceRemaining--
	}
	if !polynomialNegNumericPending(&e.numeric) && !mm.MatchesPNormalizeInvocation(&e.normalize) && !z.MatchesAddMulInvocation(&e.addMul) {
		e.active = false
	}
	return body
}

// These charged records contain diagnostics and issued child identities only.
// They never contain NUM values, pending roots, or cleanup authority.
type algebraicNegBinaryHistory struct {
	invocation                                 uint64
	constructed, sourceRemaining, ownRemaining uint8
	normalize                                  unumeral.IntegerNormalizeInvocationReceipt
	addMul                                     unumeral.IntegerAddMulInvocationReceipt
	bodyCause, sourceCause                     error
}

func (q *algebraicNegScope) binaryDiagnostic() algebraicNegBinaryHistory {
	e := &q.eval
	return algebraicNegBinaryHistory{e.invocation, e.constructed, e.sourceRemaining, e.ownRemaining, e.normalize, e.addMul, e.bodyCause, e.sourceCause}
}

type algebraicNegHistory struct {
	target                                            Algebraic
	falseAcknowledged                                 bool
	evaluations                                       [2]algebraicNegBinaryHistory
	invocation                                        uint64
	phase, constructed, sourceRemaining, ownRemaining uint8
	evalInvocation                                    uint64
	normalize, evalNormalize                          unumeral.IntegerNormalizeInvocationReceipt
	addMul                                            unumeral.IntegerAddMulInvocationReceipt
	bodyCause, sourceCause, cleanupCause              error
}

func (q *algebraicNegScope) diagnostic() algebraicNegHistory {
	h := algebraicNegHistory{target: q.target, falseAcknowledged: q.falseAcknowledged, evaluations: q.evalHistory, invocation: q.invocation, phase: q.phase, constructed: q.eval.constructed, sourceRemaining: q.eval.sourceRemaining, ownRemaining: q.eval.ownRemaining, evalInvocation: q.eval.invocation, normalize: q.normalize, evalNormalize: q.eval.normalize, addMul: q.eval.addMul, bodyCause: q.bodyCause, sourceCause: q.sourceCause, cleanupCause: q.cleanupCause}
	if q.eval.invocation >= 1 && q.eval.invocation <= 2 {
		h.evaluations[q.eval.invocation-1] = q.binaryDiagnostic()
	}
	return h
}

func (q *algebraicNegScope) updateHistory() {
	if q.historyRecorded {
		q.owner.negationHistory[q.historyIndex] = q.diagnostic()
	}
}
func (q *algebraicNegScope) prepareHistory() error {
	if q.historyRecorded {
		return nil
	}
	m := q.owner
	q.historyIndex = len(m.negationHistory)
	q.historyRecorded = true
	m.negationHistory = append(m.negationHistory, q.diagnostic())
	return nil
}
func (m *AlgebraicRootCore) negationHistoryFor(r *algebraicNegInvocationReceipt) (algebraicNegHistory, bool) {
	if m == nil || m.self != m || m.closed || r == nil || r.owner != m || r.frame != &m.negation || r.invocation == 0 {
		return algebraicNegHistory{}, false
	}
	if r.invocation == m.negation.invocation {
		return m.negation.diagnostic(), true
	}
	for _, h := range m.negationHistory {
		if h.invocation == r.invocation {
			return h, true
		}
	}
	return algebraicNegHistory{}, false
}

func numericIntegerManager_algebraicNegCallerExecuting(z *unumeral.IntegerManager) bool {
	return z != nil && z.ValidOwner() && algebraicNegCaller(z) != nil && algebraicNegCaller(z).negationExecuting() && algebraicNegCaller(z).config.Rationals.Integers() == z
}

// The input token is a borrow captured at the actual invocation, never an INC
// or owning root. It only guards the older caller-value destruction frontier.
func (m *AlgebraicRootCore) negationTargetLive(q *algebraicNegScope) bool {
	a := q.target
	return a.owner == m.origin && a.slot != 0 && uint64(a.slot) <= uint64(len(m.locations)) && m.locations[a.slot-1].live && m.locations[a.slot-1].generation == a.generation
}

func (m *AlgebraicRootCore) negationProtectsCaller(a *Algebraic) bool {
	if m == nil || m.self != m || a == nil {
		return false
	}
	q := &m.negation
	return q.active && q.sourceCause != nil && !q.falseAcknowledged && *a == q.target
}
