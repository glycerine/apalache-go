package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyRemoveOneHalfRootScope struct {
	constructed                                           uint8
	target                                                UPolynomialView
	output                                                UPolynomial
	falseAcknowledged                                     bool
	owner                                                 *UPolynomialManager
	invocation                                            uint64
	values                                                [1]unumeral.Int
	numeric                                               polynomialNumericRoots
	failedRowStore                                        *UPolynomialCoefficientStore
	failedRowBacking, failedRowGeneration, failedRowIndex uint32
	failedRowCell                                         uint64
	failedRowAcknowledged                                 bool
	normalize                                             unumeral.IntegerNormalizeInvocationReceipt
	assertion                                             upolyOneHalfRootInvocationReceipt
	addMul                                                unumeral.IntegerAddMulInvocationReceipt
	divisionRaw, extendedRaw                              unumeral.IntegerRawInvocationReceipt

	sourceRemaining, ownRemaining            uint8
	active, executing, finishing, ownStarted bool
	bodyCause, sourceCause, ownCause         error
	history                                  *upolyRemoveOneHalfRootHistory
}

// Diagnostics contain issued child tuples, never NUM payload/owning authority.
type upolyRemoveOneHalfRootHistory struct {
	constructed                                           uint8
	failedRowStore                                        *UPolynomialCoefficientStore
	failedRowBacking, failedRowGeneration, failedRowIndex uint32
	failedRowCell                                         uint64
	output                                                UPolynomial
	owner                                                 *UPolynomialManager
	frame                                                 *upolyRemoveOneHalfRootScope
	next                                                  *upolyRemoveOneHalfRootHistory
	invocation                                            uint64
	normalize                                             unumeral.IntegerNormalizeInvocationReceipt
	assertion                                             upolyOneHalfRootInvocationReceipt
	addMul                                                unumeral.IntegerAddMulInvocationReceipt
	divisionRaw, extendedRaw                              unumeral.IntegerRawInvocationReceipt

	bodyCause, sourceCause, ownCause error
	sourceRemaining                  uint8
}

func (m *UPolynomialManager) removeOneHalfRootExecuting() bool {
	return m != nil && m.self == m && m.removeOneHalfRoot.owner == m && m.removeOneHalfRoot.active && (m.removeOneHalfRoot.executing || m.removeOneHalfRoot.finishing)
}
func numericIntegerManager_removeOneHalfRootCallerExecuting(z *unumeral.IntegerManager) bool {
	return z != nil && z.ValidOwner() && removeOneHalfRootCaller(z) != nil && removeOneHalfRootCaller(z).config.Integer == z && removeOneHalfRootCaller(z).removeOneHalfRootExecuting()
}
func (c integerAllocationInvocationContext) removeOneHalfRootFrame(z *unumeral.IntegerManager) *upolyRemoveOneHalfRootScope {
	q := c.removeOneHalfRoot
	if z == nil || !z.ValidOwner() || q == nil || q.owner == nil || q.owner.self != q.owner || q != &q.owner.removeOneHalfRoot || q.owner.config.Integer != z || removeOneHalfRootCaller(z) != q.owner || q.owner.callbackDepth != 0 || !q.active || !q.executing || c.removeOneHalfRootInvocation == 0 || c.removeOneHalfRootInvocation != q.invocation {
		return nil
	}
	if q.history != nil && (q.history.owner != q.owner || q.history.frame != q || q.history.invocation != q.invocation) {
		return nil
	}
	return q
}
func (q *upolyRemoveOneHalfRootScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{removeOneHalfRoot: q, removeOneHalfRootInvocation: q.invocation}
}
func (q *upolyRemoveOneHalfRootScope) prepareHistory() error {
	if q.history != nil {
		return nil
	}
	n := uint64(reflect.TypeOf(upolyRemoveOneHalfRootHistory{}).Size())
	if e := q.owner.config.Integer.Budget().Reserve(n); e != nil {
		return e
	}
	q.history = &upolyRemoveOneHalfRootHistory{owner: q.owner, frame: q, next: q.owner.removeOneHalfRootHistory, invocation: q.invocation}
	q.owner.removeOneHalfRootHistory = q.history
	q.owner.removeOneHalfRootHistoryBytes += n
	q.updateHistory()
	return nil
}
func (q *upolyRemoveOneHalfRootScope) updateHistory() {
	if q.history != nil {
		h := q.history
		h.constructed = q.constructed
		h.output = q.output
		h.failedRowStore = q.failedRowStore
		h.failedRowBacking = q.failedRowBacking
		h.failedRowGeneration = q.failedRowGeneration
		h.failedRowIndex = q.failedRowIndex
		h.failedRowCell = q.failedRowCell
		h.assertion = q.assertion
		h.addMul = q.addMul
		h.normalize = q.normalize
		h.divisionRaw = q.divisionRaw
		h.extendedRaw = q.extendedRaw

		h.bodyCause = q.bodyCause
		h.sourceCause = q.sourceCause
		h.ownCause = q.ownCause
		h.sourceRemaining = q.sourceRemaining
	}
}
func (q *upolyRemoveOneHalfRootScope) pending() bool {
	z := q.owner.config.Integer
	mm := q.owner.modular
	return (q.failedRowStore != nil && !q.failedRowAcknowledged) || (q.assertion.frame != nil && q.assertion.frame.invocation == q.assertion.invocation && q.assertion.frame.active) || z.MatchesAddMulInvocation(&q.addMul) || q.sourceRemaining != 0 || polynomialNegNumericPending(&q.numeric) || mm.MatchesPNormalizeInvocation(&q.normalize) || z.MatchesRawInvocation(&q.divisionRaw) || z.MatchesRawInvocation(&q.extendedRaw)
}
func (q *upolyRemoveOneHalfRootScope) finish(body error) error {
	if q.owner == nil || q.owner.self != q.owner || q != &q.owner.removeOneHalfRoot || q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	defer func() { q.finishing = false; q.updateHistory() }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	z := q.owner.config.Integer
	if q.sourceCause == nil && q.assertion.frame != nil && q.assertion.frame.invocation == q.assertion.invocation {
		q.sourceCause = q.assertion.frame.sourceCause
	}
	if q.sourceCause == nil {
		q.sourceCause = q.numericChildCause()
	}
	if q.sourceCause == nil {
		for q.sourceRemaining != 0 {
			if _, e := upolynomialSourceNumeralDel(z, &q.values[q.sourceRemaining-1]); e != nil {
				q.sourceCause = e
				break
			}
			q.sourceRemaining--
		}
	}
	if !q.pending() {
		q.active = false
	}
	return errors.Join(body, q.sourceCause)
}

// CleanupRemoveOneHalfRoot owns the exact assertion and div_rem_core caller.
// Completed younger children precede the reached NUM suffix; unjoined children follow it.
func (m *UPolynomialManager) CleanupRemoveOneHalfRoot() error {
	if e := m.cleanupRemoveOneHalfRoot(false); e != nil {
		return e
	}
	return m.cleanupRemoveOneHalfRoot(true)
}

type upolyRemoveOneHalfRootInvocationReceipt struct {
	owner      *UPolynomialManager
	frame      *upolyRemoveOneHalfRootScope
	invocation uint64
	history    *upolyRemoveOneHalfRootHistory
}

func (m *UPolynomialManager) removeOneHalfRootReceipt() upolyRemoveOneHalfRootInvocationReceipt {
	q := &m.removeOneHalfRoot
	return upolyRemoveOneHalfRootInvocationReceipt{m, q, q.invocation, q.history}
}
func (m *UPolynomialManager) cleanupRemoveOneHalfRoot(abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	r := m.removeOneHalfRootReceipt()
	return m.cleanupRemoveOneHalfRootWithReceipt(&r, abort)
}
func (m *UPolynomialManager) cleanupRemoveOneHalfRootWithReceipt(r *upolyRemoveOneHalfRootInvocationReceipt, abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.removeOneHalfRoot
	if r == nil || r.owner != m || r.frame != q || r.invocation == 0 || r.invocation != q.invocation || r.history != q.history {
		return nil
	}
	if q.history != nil && (q.history.owner != m || q.history.frame != q) {
		return ErrUPolynomialOwner
	}
	if !q.active {
		return nil
	}
	if q.owner != m || q.invocation == 0 || q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	z := m.config.Integer
	old := removeOneHalfRootCaller(z)
	setRemoveOneHalfRootCaller(z, m)
	oldNatural := removeOneHalfRootCaller(z.Naturals())
	setRemoveOneHalfRootCaller(z.Naturals(), m)
	defer func() {
		setRemoveOneHalfRootCaller(z.Naturals(), oldNatural)
		setRemoveOneHalfRootCaller(z, old)
		q.finishing = false
		q.updateHistory()
	}()
	stop := func(e error) error {
		if e != nil && q.ownCause == nil {
			q.ownCause = e
		}
		return e
	}
	if q.assertion.frame != nil {
		if e := m.cleanupOneHalfRootWithReceipt(&q.assertion, false); e != nil {
			return stop(e)
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, false); e != nil {
		return stop(e)
	}
	if e := q.cleanupNumericChildren(false); e != nil {
		return stop(e)
	}
	if !q.ownStarted {
		q.ownRemaining = q.constructed
		q.ownStarted = true
	}
	for q.ownRemaining != 0 {
		if e := z.CleanupRawNumeral(&q.values[q.ownRemaining-1]); e != nil {
			return stop(e)
		}
		q.ownRemaining--
	}
	q.falseAcknowledged = true
	if !abort {
		return nil
	}
	if q.assertion.frame != nil {
		if e := m.cleanupOneHalfRootWithReceipt(&q.assertion, true); e != nil {
			return stop(e)
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return stop(e)
	}
	if e := q.cleanupNumericChildren(true); e != nil {
		return stop(e)
	}
	if e := q.acknowledgeFailedRow(); e != nil {
		return stop(e)
	}
	if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
		return stop(e)
	}
	q.active = false
	return nil
}
func (m *UPolynomialManager) beginRemoveOneHalfRoot() (*upolyRemoveOneHalfRootScope, error) {
	q := &m.removeOneHalfRoot
	if q.active {
		return nil, ErrUPolynomialCleanupPending
	}
	if q.invocation == math.MaxUint64 {
		return nil, arena.ErrBounds
	}
	*q = upolyRemoveOneHalfRootScope{owner: m, invocation: q.invocation + 1, active: true, executing: true, sourceRemaining: 0, constructed: 0}
	// First issued diagnostic storage is already charged in the owner header.
	// Later allocating calls acquire stable records only at actual leaf branches.
	if m.removeOneHalfRootFirstHistory.invocation == 0 {
		h := &m.removeOneHalfRootFirstHistory
		*h = upolyRemoveOneHalfRootHistory{owner: m, frame: q, invocation: q.invocation, next: m.removeOneHalfRootHistory}
		m.removeOneHalfRootHistory = h
		q.history = h
		q.updateHistory()
	}
	return q, nil
}

func (q *upolyRemoveOneHalfRootScope) matches() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.removeOneHalfRoot && q.invocation != 0 && (q.history == nil || (q.history.owner == q.owner && q.history.frame == q && q.history.invocation == q.invocation))
}
func (q *upolyRemoveOneHalfRootScope) numericChildCause() error {
	z := q.owner.config.Integer
	if e := z.AddMulInvocationSourceCause(&q.addMul); e != nil {
		return e
	}
	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.divisionRaw, &q.extendedRaw} {
		if e := z.RawInvocationSourceCause(r); e != nil {
			return e
		}
	}
	return nil
}
func (q *upolyRemoveOneHalfRootScope) cleanupNumericChildren(abort bool) error {
	z := q.owner.config.Integer
	if e := z.CleanupAddMulInvocation(&q.addMul, abort); e != nil {
		return e
	}
	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.divisionRaw, &q.extendedRaw} {
		if e := z.CleanupRawInvocation(r, abort); e != nil {
			return e
		}
	}

	return nil
}
func (c integerAllocationInvocationContext) recordRemoveOneHalfRaw(z *unumeral.IntegerManager, r *unumeral.IntegerRawInvocationReceipt) {
	q := c.removeOneHalfRootFrame(z)
	if q == nil || !z.MatchesRawInvocation(r) {
		return
	}

	switch r.Kind() {
	case unumeral.IntegerRawDivision:
		q.divisionRaw = *r
	case unumeral.IntegerRawExtendedGCD:
		q.extendedRaw = *r
	}
}

func numericNaturalManager_removeOneHalfRootCallerExecuting(n *unumeral.NaturalManager) bool {
	return n != nil && n.ValidOwner() && removeOneHalfRootCaller(n) != nil && removeOneHalfRootCaller(n).self == removeOneHalfRootCaller(n) && removeOneHalfRootCaller(n).config.Integer != nil && removeOneHalfRootCaller(n).config.Integer.Naturals() == n && removeOneHalfRootCaller(n).removeOneHalfRootExecuting()
}

// The lexical am source suffix runs at the actual div_rem_core return, before
// the outer div reset/swap. Source destruction remains distinct from own abort.
func (q *upolyRemoveOneHalfRootScope) finishAmSource(body error) error {
	if !q.matches() || !q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	defer func() { q.finishing = false; q.updateHistory() }()
	if q.sourceCause == nil {
		q.sourceCause = q.numericChildCause()
	}
	if q.sourceCause == nil && q.sourceRemaining != 0 {
		if _, e := upolynomialSourceNumeralDel(q.owner.config.Integer, &q.values[0]); e != nil {
			q.sourceCause = e
		} else {
			q.sourceRemaining = 0
		}
	}
	return errors.Join(body, q.sourceCause)
}
func removeOrHenselAddMulChild(context integerAllocationInvocationContext, r *upolyHenselPairInvocationReceipt, c *upolyHenselCallerScope) *unumeral.IntegerAddMulInvocationReceipt {
	if n := context.berlekamp; n != nil && n.NumericBound(true) && context.berlekampGeneration == n.generation {
		return &n.addMul
	}
	if q := context.removeOneHalfRoot; q != nil && q.owner != nil && context.removeOneHalfRootFrame(q.owner.config.Integer) != nil {
		return &q.addMul
	}
	return henselNumericAddMulChild(r, c)
}

// A failed source SET can publish this owner's dead allocation token into a
// constructed buffer NUM before SIZE. Owning acknowledgment clears only that
// captured exact poison cell; it does not delete a foreign replacement NUM.
func (q *upolyRemoveOneHalfRootScope) recordFailedRow(s *upolyVectorState, i uint32) {
	if !q.matches() || !q.executing || s != &q.owner.div1 || s.backing == 0 || uint64(i) >= uint64(len(q.owner.values(s))) {
		return
	}
	row := &q.owner.values(s)[i]
	if !q.owner.config.Integer.IsFailureNumeral(row) {
		return
	}
	q.failedRowStore = q.owner.stateStore(s)
	q.failedRowBacking = s.backing
	q.failedRowGeneration = s.generation
	q.failedRowIndex = i
	q.failedRowCell = row.NativeCellToken()
}
func (q *upolyRemoveOneHalfRootScope) acknowledgeFailedRow() error {
	if q.failedRowStore == nil || q.failedRowAcknowledged {
		return nil
	}
	s := q.failedRowStore
	if s.self != s || q.failedRowBacking == 0 || uint64(q.failedRowBacking) > uint64(len(s.backings)) {
		return ErrUPolynomialOwner
	}
	b := &s.backings[q.failedRowBacking-1]
	if b.live && b.generation == q.failedRowGeneration && uint64(q.failedRowIndex) < uint64(len(b.values)) {
		row := &b.values[q.failedRowIndex]
		if row.NativeCellToken() == q.failedRowCell && q.owner.config.Integer.IsFailureNumeral(row) {
			q.owner.config.Integer.AcknowledgeFailedNumeral(row, q.failedRowCell)
		}
	}
	q.failedRowAcknowledged = true
	return nil
}
