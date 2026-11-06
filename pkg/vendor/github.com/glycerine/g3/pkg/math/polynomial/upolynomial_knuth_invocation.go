package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1594-1602,2029-2074.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The source has no scoped NUMs here. Only actual borrowed coefficient storage,
// direct allocation leaves, and reached PN children belong to this caller frame.
type upolyKnuthScope struct {
	owner                                           *UPolynomialManager
	invocation                                      uint64
	target                                          UPolynomialView
	kind, stage                                     uint8
	cursor                                          uint32
	active, executing, finishing, falseAcknowledged bool
	numeric                                         polynomialNumericRoots
	normalize                                       unumeral.IntegerNormalizeInvocationReceipt
	bodyCause, sourceCause, ownCause                error
	history                                         *upolyKnuthHistory
}
type upolyKnuthHistory struct {
	target                           UPolynomialView
	owner                            *UPolynomialManager
	frame                            *upolyKnuthScope
	next                             *upolyKnuthHistory
	invocation                       uint64
	kind, stage                      uint8
	cursor                           uint32
	normalize                        unumeral.IntegerNormalizeInvocationReceipt
	bodyCause, sourceCause, ownCause error
}
type upolyKnuthInvocationReceipt struct {
	owner      *UPolynomialManager
	frame      *upolyKnuthScope
	invocation uint64
	history    *upolyKnuthHistory
}

func (m *UPolynomialManager) knuthExecuting() bool {
	return m != nil && m.self == m && m.knuth.owner == m && m.knuth.active && (m.knuth.executing || m.knuth.finishing)
}
func numericIntegerManager_knuthCallerExecuting(z *unumeral.IntegerManager) bool {
	return z != nil && z.ValidOwner() && knuthCaller(z) != nil && knuthCaller(z).config.Integer == z && knuthCaller(z).knuthExecuting()
}
func numericNaturalManager_knuthCallerExecuting(n *unumeral.NaturalManager) bool {
	return n != nil && n.ValidOwner() && knuthCaller(n) != nil && knuthCaller(n).config.Integer != nil && knuthCaller(n).config.Integer.Naturals() == n && knuthCaller(n).knuthExecuting()
}
func (c integerAllocationInvocationContext) knuthFrame(z *unumeral.IntegerManager) *upolyKnuthScope {
	q := c.knuth
	if z == nil || !z.ValidOwner() || q == nil || q.owner == nil || q.owner.self != q.owner || q != &q.owner.knuth || q.owner.config.Integer != z || knuthCaller(z) != q.owner || q.owner.callbackDepth != 0 || !q.active || !q.executing || c.knuthInvocation == 0 || c.knuthInvocation != q.invocation {
		return nil
	}
	return q
}
func (q *upolyKnuthScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{knuth: q, knuthInvocation: q.invocation}
}
func (q *upolyKnuthScope) updateHistory() {
	if h := q.history; h != nil {
		h.target = q.target
		h.kind = q.kind
		h.stage = q.stage
		h.cursor = q.cursor
		h.normalize = q.normalize
		h.bodyCause = q.bodyCause
		h.sourceCause = q.sourceCause
		h.ownCause = q.ownCause
	}
}
func (q *upolyKnuthScope) prepareHistory() error {
	if q.history != nil {
		return nil
	}
	n := uint64(reflect.TypeOf(upolyKnuthHistory{}).Size())
	if e := q.owner.config.Integer.Budget().Reserve(n); e != nil {
		return e
	}
	q.history = &upolyKnuthHistory{owner: q.owner, frame: q, invocation: q.invocation, next: q.owner.knuthHistory}
	q.owner.knuthHistory = q.history
	q.owner.knuthHistoryBytes += n
	q.updateHistory()
	return nil
}
func (m *UPolynomialManager) beginKnuth(v UPolynomialView, kind uint8) (*upolyKnuthScope, error) {
	q := &m.knuth
	if q.active {
		return nil, ErrUPolynomialCleanupPending
	}
	if q.invocation == math.MaxUint64 {
		return nil, arena.ErrBounds
	}
	*q = upolyKnuthScope{owner: m, invocation: q.invocation + 1, target: v, kind: kind, active: true, executing: true}
	return q, nil
}
func (q *upolyKnuthScope) pending() bool {
	return polynomialNegNumericPending(&q.numeric) || q.owner.modular.MatchesPNormalizeInvocation(&q.normalize)
}
func (q *upolyKnuthScope) finish(body error) error {
	if q.owner == nil || q.owner.self != q.owner || q != &q.owner.knuth || q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	defer func() { q.finishing = false; q.updateHistory() }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if !q.pending() {
		q.active = false
	}
	return errors.Join(body, q.sourceCause)
}
func (m *UPolynomialManager) knuthReceipt() upolyKnuthInvocationReceipt {
	q := &m.knuth
	return upolyKnuthInvocationReceipt{m, q, q.invocation, q.history}
}
func (m *UPolynomialManager) CleanupKnuthBounds() error {
	if e := m.cleanupKnuth(false); e != nil {
		return e
	}
	return m.cleanupKnuth(true)
}
func (m *UPolynomialManager) cleanupKnuth(abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	r := m.knuthReceipt()
	return m.cleanupKnuthWithReceipt(&r, abort)
}
func (m *UPolynomialManager) cleanupKnuthWithReceipt(r *upolyKnuthInvocationReceipt, abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.knuth
	if r == nil || r.owner != m || r.frame != q || r.invocation == 0 || r.invocation != q.invocation || r.history != q.history {
		return nil
	}
	if !q.active {
		return nil
	}
	if q.owner != m || q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	z := m.config.Integer
	old, oldN := knuthCaller(z), knuthCaller(z.Naturals())
	setKnuthCaller(z, m)
	setKnuthCaller(z.Naturals(), m)
	defer func() {
		setKnuthCaller(z, old)
		setKnuthCaller(z.Naturals(), oldN)
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
	q.falseAcknowledged = true
	if !abort {
		return nil
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return stop(e)
	}
	if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
		return stop(e)
	}
	q.active = false
	return nil
}
