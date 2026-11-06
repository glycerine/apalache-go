package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1906-1951.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyRootUpperBoundScope struct {
	constructed              uint8
	target                   UPolynomialView
	falseAcknowledged        bool
	owner                    *UPolynomialManager
	invocation               uint64
	values                   [5]unumeral.Int
	numeric                  polynomialNumericRoots
	normalize                unumeral.IntegerNormalizeInvocationReceipt
	divisionRaw, extendedRaw unumeral.IntegerRawInvocationReceipt

	sourceRemaining, ownRemaining            uint8
	active, executing, finishing, ownStarted bool
	bodyCause, sourceCause, ownCause         error
	history                                  *upolyRootUpperBoundHistory
}

// Diagnostics contain issued child tuples, never NUM payload/owning authority.
type upolyRootUpperBoundHistory struct {
	constructed              uint8
	owner                    *UPolynomialManager
	frame                    *upolyRootUpperBoundScope
	next                     *upolyRootUpperBoundHistory
	invocation               uint64
	normalize                unumeral.IntegerNormalizeInvocationReceipt
	divisionRaw, extendedRaw unumeral.IntegerRawInvocationReceipt

	bodyCause, sourceCause, ownCause error
	sourceRemaining                  uint8
}

func (m *UPolynomialManager) rootUpperBoundExecuting() bool {
	return m != nil && m.self == m && m.rootUpperBound.owner == m && m.rootUpperBound.active && (m.rootUpperBound.executing || m.rootUpperBound.finishing)
}
func numericIntegerManager_rootUpperBoundCallerExecuting(z *unumeral.IntegerManager) bool {
	return z != nil && z.ValidOwner() && rootUpperBoundCaller(z) != nil && rootUpperBoundCaller(z).config.Integer == z && rootUpperBoundCaller(z).rootUpperBoundExecuting()
}
func (c integerAllocationInvocationContext) rootUpperBoundFrame(z *unumeral.IntegerManager) *upolyRootUpperBoundScope {
	q := c.rootUpperBound
	if z == nil || !z.ValidOwner() || q == nil || q.owner == nil || q.owner.self != q.owner || q != &q.owner.rootUpperBound || q.owner.config.Integer != z || rootUpperBoundCaller(z) != q.owner || q.owner.callbackDepth != 0 || !q.active || !q.executing || q.constructed < 4 || c.rootUpperBoundInvocation == 0 || c.rootUpperBoundInvocation != q.invocation {
		return nil
	}
	if q.history != nil && (q.history.owner != q.owner || q.history.frame != q || q.history.invocation != q.invocation) {
		return nil
	}
	return q
}
func (q *upolyRootUpperBoundScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{rootUpperBound: q, rootUpperBoundInvocation: q.invocation}
}
func (q *upolyRootUpperBoundScope) prepareHistory() error {
	if q.history != nil {
		return nil
	}
	n := uint64(reflect.TypeOf(upolyRootUpperBoundHistory{}).Size())
	if e := q.owner.config.Integer.Budget().Reserve(n); e != nil {
		return e
	}
	q.history = &upolyRootUpperBoundHistory{owner: q.owner, frame: q, next: q.owner.rootUpperBoundHistory, invocation: q.invocation}
	q.owner.rootUpperBoundHistory = q.history
	q.owner.rootUpperBoundHistoryBytes += n
	q.updateHistory()
	return nil
}
func (q *upolyRootUpperBoundScope) updateHistory() {
	if q.history != nil {
		h := q.history
		h.constructed = q.constructed
		h.normalize = q.normalize
		h.divisionRaw = q.divisionRaw
		h.extendedRaw = q.extendedRaw

		h.bodyCause = q.bodyCause
		h.sourceCause = q.sourceCause
		h.ownCause = q.ownCause
		h.sourceRemaining = q.sourceRemaining
	}
}
func (q *upolyRootUpperBoundScope) pending() bool {
	z := q.owner.config.Integer
	mm := q.owner.modular
	return q.sourceRemaining != 0 || polynomialNegNumericPending(&q.numeric) || mm.MatchesPNormalizeInvocation(&q.normalize) || z.MatchesRawInvocation(&q.divisionRaw) || z.MatchesRawInvocation(&q.extendedRaw)
}
func (q *upolyRootUpperBoundScope) finish(body error) error {
	if q.owner == nil || q.owner.self != q.owner || q != &q.owner.rootUpperBound || q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	defer func() { q.finishing = false; q.updateHistory() }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	z := q.owner.config.Integer
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

// CleanupRootUpperBound owns only the actual current public Cauchy-bound scope.
// Completed younger children precede the reached NUM suffix; unjoined children follow it.
func (m *UPolynomialManager) CleanupRootUpperBound() error {
	if e := m.cleanupRootUpperBound(false); e != nil {
		return e
	}
	return m.cleanupRootUpperBound(true)
}

type upolyRootUpperBoundInvocationReceipt struct {
	owner      *UPolynomialManager
	frame      *upolyRootUpperBoundScope
	invocation uint64
	history    *upolyRootUpperBoundHistory
}

func (m *UPolynomialManager) rootUpperBoundReceipt() upolyRootUpperBoundInvocationReceipt {
	q := &m.rootUpperBound
	return upolyRootUpperBoundInvocationReceipt{m, q, q.invocation, q.history}
}
func (m *UPolynomialManager) cleanupRootUpperBound(abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	r := m.rootUpperBoundReceipt()
	return m.cleanupRootUpperBoundWithReceipt(&r, abort)
}
func (m *UPolynomialManager) cleanupRootUpperBoundWithReceipt(r *upolyRootUpperBoundInvocationReceipt, abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.rootUpperBound
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
	old := rootUpperBoundCaller(z)
	setRootUpperBoundCaller(z, m)
	oldNatural := rootUpperBoundCaller(z.Naturals())
	setRootUpperBoundCaller(z.Naturals(), m)
	defer func() {
		setRootUpperBoundCaller(z.Naturals(), oldNatural)
		setRootUpperBoundCaller(z, old)
		q.finishing = false
		q.updateHistory()
	}()
	stop := func(e error) error {
		if e != nil && q.ownCause == nil {
			q.ownCause = e
		}
		return e
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
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return stop(e)
	}
	if e := q.cleanupNumericChildren(true); e != nil {
		return stop(e)
	}
	if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
		return stop(e)
	}
	q.active = false
	return nil
}
func (m *UPolynomialManager) beginRootUpperBound() (*upolyRootUpperBoundScope, error) {
	q := &m.rootUpperBound
	if q.active {
		return nil, ErrUPolynomialCleanupPending
	}
	if q.invocation == math.MaxUint64 {
		return nil, arena.ErrBounds
	}
	*q = upolyRootUpperBoundScope{owner: m, invocation: q.invocation + 1, active: true, executing: true, sourceRemaining: 4, constructed: 4}
	return q, nil
}

func (q *upolyRootUpperBoundScope) matches() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.rootUpperBound && q.invocation != 0 && (q.history == nil || (q.history.owner == q.owner && q.history.frame == q && q.history.invocation == q.invocation))
}
func (q *upolyRootUpperBoundScope) numericChildCause() error {
	z := q.owner.config.Integer
	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.divisionRaw, &q.extendedRaw} {
		if e := z.RawInvocationSourceCause(r); e != nil {
			return e
		}
	}
	return nil
}
func (q *upolyRootUpperBoundScope) cleanupNumericChildren(abort bool) error {
	z := q.owner.config.Integer
	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.divisionRaw, &q.extendedRaw} {
		if e := z.CleanupRawInvocation(r, abort); e != nil {
			return e
		}
	}

	return nil
}
func (c integerAllocationInvocationContext) recordRootUpperRaw(z *unumeral.IntegerManager, r *unumeral.IntegerRawInvocationReceipt) {
	q := c.rootUpperBoundFrame(z)
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

func numericNaturalManager_rootUpperBoundCallerExecuting(n *unumeral.NaturalManager) bool {
	return n != nil && n.ValidOwner() && rootUpperBoundCaller(n) != nil && rootUpperBoundCaller(n).self == rootUpperBoundCaller(n) && rootUpperBoundCaller(n).config.Integer != nil && rootUpperBoundCaller(n).config.Integer.Naturals() == n && rootUpperBoundCaller(n).rootUpperBoundExecuting()
}
