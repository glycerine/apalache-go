package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1298-1321.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyOneHalfRootScope struct {
	constructed                              uint8
	removeParent                             *upolyRemoveOneHalfRootScope
	removeParentInvocation                   uint64
	target                                   UPolynomialView
	falseAcknowledged                        bool
	owner                                    *UPolynomialManager
	invocation                               uint64
	values                                   [2]unumeral.Int
	numeric                                  polynomialNumericRoots
	normalize                                unumeral.IntegerNormalizeInvocationReceipt
	sourceRemaining, ownRemaining            uint8
	active, executing, finishing, ownStarted bool
	bodyCause, sourceCause, ownCause         error
	history                                  *upolyOneHalfRootHistory
}

// Diagnostics contain issued child tuples, never NUM payload/owning authority.
type upolyOneHalfRootHistory struct {
	target                           UPolynomialView
	constructed                      uint8
	owner                            *UPolynomialManager
	frame                            *upolyOneHalfRootScope
	next                             *upolyOneHalfRootHistory
	invocation                       uint64
	normalize                        unumeral.IntegerNormalizeInvocationReceipt
	bodyCause, sourceCause, ownCause error
	sourceRemaining                  uint8
}

func (m *UPolynomialManager) oneHalfRootExecuting() bool {
	return m != nil && m.self == m && m.oneHalfRoot.owner == m && m.oneHalfRoot.active && (m.oneHalfRoot.executing || m.oneHalfRoot.finishing)
}
func numericIntegerManager_oneHalfRootCallerExecuting(z *unumeral.IntegerManager) bool {
	return z != nil && z.ValidOwner() && oneHalfRootCaller(z) != nil && oneHalfRootCaller(z).config.Integer == z && oneHalfRootCaller(z).oneHalfRootExecuting()
}
func numericNaturalManager_oneHalfRootCallerExecuting(n *unumeral.NaturalManager) bool {
	return n != nil && n.ValidOwner() && oneHalfRootCaller(n) != nil && oneHalfRootCaller(n).config.Integer != nil && oneHalfRootCaller(n).config.Integer.Naturals() == n && oneHalfRootCaller(n).oneHalfRootExecuting()
}
func (c integerAllocationInvocationContext) oneHalfRootFrame(z *unumeral.IntegerManager) *upolyOneHalfRootScope {
	q := c.oneHalfRoot
	if z == nil || !z.ValidOwner() || q == nil || q.owner == nil || q.owner.self != q.owner || q != &q.owner.oneHalfRoot || q.owner.config.Integer != z || oneHalfRootCaller(z) != q.owner || q.owner.callbackDepth != 0 || !q.active || !q.executing || q.constructed != 2 || c.oneHalfRootInvocation == 0 || c.oneHalfRootInvocation != q.invocation {
		return nil
	}
	if q.history != nil && (q.history.owner != q.owner || q.history.frame != q || q.history.invocation != q.invocation) {
		return nil
	}
	return q
}
func (q *upolyOneHalfRootScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{oneHalfRoot: q, oneHalfRootInvocation: q.invocation}
}
func (q *upolyOneHalfRootScope) prepareHistory() error {
	if p := q.removeParent; p != nil {
		if !p.matches() || p.owner != q.owner || p.invocation != q.removeParentInvocation || !p.active || !p.executing {
			return ErrUPolynomialOwner
		}
		if e := p.prepareHistory(); e != nil {
			return e
		}
	}

	if q.history != nil {
		return nil
	}
	n := uint64(reflect.TypeOf(upolyOneHalfRootHistory{}).Size())
	if e := q.owner.config.Integer.Budget().Reserve(n); e != nil {
		return e
	}
	q.history = &upolyOneHalfRootHistory{owner: q.owner, frame: q, next: q.owner.oneHalfRootHistory, invocation: q.invocation}
	q.owner.oneHalfRootHistory = q.history
	q.owner.oneHalfRootHistoryBytes += n
	q.updateHistory()
	return nil
}
func (q *upolyOneHalfRootScope) updateHistory() {
	if q.history != nil {
		h := q.history
		h.target = q.target
		h.constructed = q.constructed
		h.normalize = q.normalize
		h.bodyCause = q.bodyCause
		h.sourceCause = q.sourceCause
		h.ownCause = q.ownCause
		h.sourceRemaining = q.sourceRemaining
	}
}
func (q *upolyOneHalfRootScope) pending() bool {
	mm := q.owner.modular
	return q.sourceRemaining != 0 || polynomialNegNumericPending(&q.numeric) || mm.MatchesPNormalizeInvocation(&q.normalize)
}
func (q *upolyOneHalfRootScope) finish(body error) error {
	if q.owner == nil || q.owner.self != q.owner || q != &q.owner.oneHalfRoot || q.executing || q.finishing {
		return ErrUPolynomialCleanupPending
	}
	q.finishing = true
	defer func() { q.finishing = false; q.updateHistory() }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	z := q.owner.config.Integer
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

// CleanupOneHalfRoot owns only the actual current public Horner scope.
// Completed PN children precede ak then r, and unjoined children follow them.
func (m *UPolynomialManager) CleanupOneHalfRoot() error {
	if e := m.cleanupOneHalfRoot(false); e != nil {
		return e
	}
	return m.cleanupOneHalfRoot(true)
}

type upolyOneHalfRootInvocationReceipt struct {
	owner      *UPolynomialManager
	frame      *upolyOneHalfRootScope
	invocation uint64
	history    *upolyOneHalfRootHistory
}

func (m *UPolynomialManager) oneHalfRootReceipt() upolyOneHalfRootInvocationReceipt {
	q := &m.oneHalfRoot
	return upolyOneHalfRootInvocationReceipt{m, q, q.invocation, q.history}
}
func (m *UPolynomialManager) cleanupOneHalfRoot(abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	r := m.oneHalfRootReceipt()
	return m.cleanupOneHalfRootWithReceipt(&r, abort)
}
func (m *UPolynomialManager) cleanupOneHalfRootWithReceipt(r *upolyOneHalfRootInvocationReceipt, abort bool) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.oneHalfRoot
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
	old, oldN := oneHalfRootCaller(z), oneHalfRootCaller(z.Naturals())
	setOneHalfRootCaller(z, m)
	setOneHalfRootCaller(z.Naturals(), m)
	defer func() {
		setOneHalfRootCaller(z, old)
		setOneHalfRootCaller(z.Naturals(), oldN)
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
	if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
		return stop(e)
	}
	q.active = false
	return nil
}
func (m *UPolynomialManager) beginOneHalfRoot() (*upolyOneHalfRootScope, error) {
	q := &m.oneHalfRoot
	if q.active {
		return nil, ErrUPolynomialCleanupPending
	}
	if q.invocation == math.MaxUint64 {
		return nil, arena.ErrBounds
	}
	*q = upolyOneHalfRootScope{owner: m, invocation: q.invocation + 1, active: true, executing: true, sourceRemaining: 2, constructed: 2}
	return q, nil
}

// The private typed seam rejects a copied/stale frame or a different real
// modular receiver before the literal numeric source action.
func numericIntegerModularManager_oneHalfRootContextValid(m *unumeral.IntegerModularManager, c integerAllocationInvocationContext) bool {
	return m != nil && m.ValidOwner() && m.Integers() != nil && c.oneHalfRootFrame(m.Integers()) != nil && c.oneHalfRoot.owner.modular == m
}
func numericIntegerModularManager_copyWithOneHalfRootContext(m *unumeral.IntegerModularManager, out, value *unumeral.Int, c integerAllocationInvocationContext) error {
	if !numericIntegerModularManager_oneHalfRootContextValid(m, c) {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Copy(out, value)
}
func numericIntegerModularManager_mul2kWithOneHalfRootContext(m *unumeral.IntegerModularManager, value *unumeral.Int, k uint32, out *unumeral.Int, c integerAllocationInvocationContext) error {
	if !numericIntegerModularManager_oneHalfRootContextValid(m, c) {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Mul2k(value, k, out)
}
func numericIntegerModularManager_addWithOneHalfRootContext(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, c integerAllocationInvocationContext) error {
	if !numericIntegerModularManager_oneHalfRootContextValid(m, c) {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Add(a, b, out)
}
