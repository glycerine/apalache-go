package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1108-1225;
// src/util/mpz.cpp:1385-1410; src/util/scoped_numeral.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyDisplayScope struct {
	owner                                            *UPolynomialManager
	previousInteger                                  *upolyDisplayScope
	parent, next                                     *upolyDisplayScope
	header                                           uint64
	previousCallbackDepth                            uint32
	invocation                                       uint64
	active, executing, ownExecuting, sourceExecuting bool
	smt, constructed, sourceEnded                    bool
	numeral                                          unumeral.Int
	insertion                                        unumeral.NumeralInsertionReceipt
	characters                                       unumeral.IntegerCharacterScope
	characterFormatting                              unumeral.NaturalFormattingScope
	characterCaller                                  unumeral.NumericCharacterCaller
	normalize                                        unumeral.IntegerNormalizeInvocationReceipt
	numeric                                          polynomialNumericRoots
	bodyCause, sourceCause, ownCause                 error
}

func (m *UPolynomialManager) beginDisplay(smt bool) (*upolyDisplayScope, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	q := &m.display
	parent := m.currentDisplay
	if parent != nil {
		q = &upolyDisplayScope{}
	} else if q.executing || q.ownExecuting || q.sourceExecuting {
		return nil, ErrUPolynomialCleanupPending
	} else if q.active {
		// Retained older RAW authority is independent of this literal call.
		// Inline erased routes need no persistent allocation or old-frame reset.
		q = &upolyDisplayScope{}
	}
	if m.displayNonce == ^uint64(0) {
		return nil, arena.ErrBounds
	}
	m.displayNonce++
	*q = upolyDisplayScope{owner: m, invocation: m.displayNonce, active: true, executing: true, smt: smt, parent: parent}
	q.previousCallbackDepth = m.callbackDepth
	m.callbackDepth = 0
	m.currentDisplay = q
	q.enterCall()
	return q, nil
}
func (q *upolyDisplayScope) actual() bool {
	if q == nil || q.owner == nil || q.owner.self != q.owner || q.invocation == 0 {
		return false
	}
	m := q.owner
	if q == &m.display {
		return true
	}
	for f := m.currentDisplay; f != nil; f = f.parent {
		if f == q {
			return true
		}
	}
	for f := m.displayFrames; f != nil; f = f.next {
		if f == q && f.header != 0 {
			return true
		}
	}
	return false
}

// Extra persistent call storage is charged only at an actual allocation
// frontier. Inline nested calls use their literal call-local metadata and do
// not reserve a diagnostic archive ahead of erased numerical branches.
func (q *upolyDisplayScope) prepareStorage() error {
	if !q.actual() {
		return ErrUPolynomialOwner
	}
	if q == &q.owner.display || q.header != 0 {
		return nil
	}
	bytes := uint64(reflect.TypeOf(upolyDisplayScope{}).Size())
	if e := q.owner.config.Integer.Budget().Reserve(bytes); e != nil {
		return e
	}
	q.header = bytes
	q.next = q.owner.displayFrames
	q.owner.displayFrames = q
	return nil
}
func (q *upolyDisplayScope) endCall() {
	q.leaveCall()
	q.owner.currentDisplay = q.parent
	q.owner.callbackDepth = q.previousCallbackDepth
	if q.parent != nil && q.sourceCause != nil && q.parent.sourceCause == nil {
		q.parent.sourceCause = q.sourceCause
	}
	// An exited inline RAW value has no source heap acquisition and no DEL.
	// Persist only actual allocating scopes, whose cold storage was charged.
	if q != &q.owner.display && q.header == 0 {
		q.active = false
	}
}
func (q *upolyDisplayScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{upolyDisplay: q, upolyDisplayInvocation: q.invocation}
}
func (c integerAllocationInvocationContext) upolyDisplayFrame(z *unumeral.IntegerManager) *upolyDisplayScope {
	q := c.upolyDisplay
	if !q.actual() || q.owner.config.Integer != z || z == nil || !z.ValidOwner() || z.RuntimeOwner().FailureTransportActive() || !q.active || !q.executing || !q.constructed || q.owner.callbackDepth != 0 || q.owner.currentDisplay != q || c.upolyDisplayInvocation != q.invocation {
		return nil
	}
	return q
}
func (q *upolyDisplayScope) insert(out unumeral.RationalSourceStream, a *unumeral.Int) error {
	q.characterCaller = unumeral.NumericCharacterCaller{Characters: &q.characters, Formatting: &q.characterFormatting, Generation: q.invocation, Policy: q}
	e := q.owner.config.Integer.InsertSourceIntegerTracked(out, a, &q.insertion, &q.characterCaller)
	if q.insertion.SourceCause() != nil && q.sourceCause == nil {
		q.sourceCause = q.insertion.SourceCause()
	}
	return e
}
func (q *upolyDisplayScope) finish(body error) error {
	if !q.actual() || q.executing || q.ownExecuting || q.sourceExecuting {
		return upolyDisplayJoin(body, ErrUPolynomialCleanupPending)
	}
	if body != nil && q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause != nil {
		return upolyDisplayJoin(body, q.sourceCause)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	// RAW abs_n has an explicit successful-body DEL inside smtNumeral only.
	if !q.smt && q.constructed {
		_, e := upolynomialSourceNumeralDel(q.owner.config.Integer, &q.numeral)
		if e != nil {
			q.sourceCause = e
			return upolyDisplayJoin(body, e)
		}
		q.constructed = false
	}
	q.sourceEnded = true
	if !q.constructed && !q.insertion.Active() && !polynomialNegNumericPending(&q.numeric) {
		q.active = false
	}
	return body
}
func (m *UPolynomialManager) CleanupDisplay() error {
	if m != nil && m.factorTrace != nil && m.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.factorsDisplay != nil && m.factorsDisplay.NumericExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if m.displayExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if e := m.cleanupDisplayFrame(&m.display); e != nil {
		return e
	}
	for q := m.displayFrames; q != nil; q = q.next {
		if e := m.cleanupDisplayFrame(q); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) cleanupDisplayFrame(q *upolyDisplayScope) (cause error) {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if !q.active {
		return nil
	}
	if !q.actual() || q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.enterCall()
	defer q.leaveCall()
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	if q.insertion.Active() {
		if e := q.insertion.Complete(m.config.Integer.Naturals(), false); e != nil {
			return e
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, false); e != nil {
		return e
	}
	// A stopped younger source child protects NUM a on source exit; owning abort
	// disposes that exact older caller field before unjoined child allocations.
	if q.constructed {
		if e := m.config.Integer.CleanupRawNumeral(&q.numeral); e != nil {
			return e
		}
		q.constructed = false
	}
	if q.insertion.Active() {
		if e := q.insertion.Complete(m.config.Integer.Naturals(), true); e != nil {
			return e
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return e
	}
	if e := cleanupUPolynomialNumericRoots(m.config.Integer, &q.numeric); e != nil {
		return e
	}
	q.active = false
	return nil
}

func (m *UPolynomialManager) displayExecuting() bool {
	if m == nil || m.self != m {
		return false
	}
	for f := m.displayFrames; f != nil; f = f.next {
		if f.actual() && (f.executing || f.sourceExecuting || f.ownExecuting) {
			return true
		}
	}
	q := m.currentDisplay
	if q == nil {
		q = &m.display
	}
	return (q.actual() && (q.executing || q.sourceExecuting || q.ownExecuting)) || (m.displaySequence != nil && m.displaySequence.self == m.displaySequence && m.displaySequence.manager == m && m.displaySequence.displaying)
}

// The caller chain excludes owner retirement only; numeric acquisition still
// requires the explicit captured invocation context at each reached leaf.
func (q *upolyDisplayScope) enterCall() {
	z := q.owner.config.Integer
	q.previousInteger = upolyDisplayCaller(z)
	setUpolyDisplayCaller(z, q)
}
func (q *upolyDisplayScope) leaveCall() {
	setUpolyDisplayCaller(q.owner.config.Integer, q.previousInteger)
	q.previousInteger = nil
}
func numericIntegerManager_upolyDisplayCallerExecuting(z *unumeral.IntegerManager) bool {
	if z == nil || !z.ValidOwner() {
		return false
	}
	if seq := upolyDisplaySequenceCaller(z); seq != nil && seq.self == seq && seq.manager != nil && seq.manager.self == seq.manager && seq.manager.config.Integer == z && seq.manager.displaySequence == seq && seq.displaying {
		return true
	}
	for q := upolyDisplayCaller(z); q != nil; q = q.previousInteger {
		if q.actual() && q.owner.config.Integer == z && (q.executing || q.sourceExecuting || q.ownExecuting) {
			return true
		}
	}
	return false
}

// Each output operation borrows the stream only for its actual transport call.
// Foreign callback numerical calls cannot adopt this executing display context.
type upolyDisplayStream struct {
	scope *upolyDisplayScope
	out   unumeral.RationalSourceStream
}

func (s upolyDisplayStream) AppendInt32(v int32) error {
	s.scope.owner.callbackDepth++
	defer func() { s.scope.owner.callbackDepth-- }()
	return s.out.AppendInt32(v)
}
func (s upolyDisplayStream) AppendCString(v string) error {
	s.scope.owner.callbackDepth++
	defer func() { s.scope.owner.callbackDepth-- }()
	return s.out.AppendCString(v)
}
func (s upolyDisplayStream) AppendNumeralText(v string) error {
	s.scope.owner.callbackDepth++
	defer func() { s.scope.owner.callbackDepth-- }()
	return s.out.AppendNumeralText(v)
}

func numericIntegerModularManager_copyWithDisplayContext(m *unumeral.IntegerModularManager, out, value *unumeral.Int, c integerAllocationInvocationContext) error {
	if m == nil || m.Integers() == nil {
		return unumeral.ErrModularOwner
	}
	q := c.upolyDisplayFrame(m.Integers())
	if q == nil || q.owner.modular != m {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Copy(out, value)
}
func numericIntegerModularManager_negWithDisplayContext(m *unumeral.IntegerModularManager, out *unumeral.Int, c integerAllocationInvocationContext) error {
	if m == nil || m.Integers() == nil {
		return unumeral.ErrModularOwner
	}
	q := c.upolyDisplayFrame(m.Integers())
	if q == nil || q.owner.modular != m {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Neg(out)
}

func (q *upolyDisplayScope) NumericCharacterValid(n *unumeral.NaturalManager, executing bool) bool {
	if !q.actual() || !q.active || q.owner.config.Integer.Naturals() != n {
		return false
	}
	return !executing || q.executing && q.owner.currentDisplay == q && q.owner.callbackDepth == 0 && !q.owner.config.Integer.RuntimeOwner().FailureTransportActive()
}
func (q *upolyDisplayScope) NumericCharacterPrepare() error { return q.prepareStorage() }

// Ownership is granted only by an accepted beginDisplay, including the exact
// frame used when an older independently retained display already exists.
type upolyDisplayInvocationReceipt struct {
	owner      *UPolynomialManager
	frame      *upolyDisplayScope
	invocation uint64
}

func (m *UPolynomialManager) matchesDisplayReceipt(r *upolyDisplayInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.frame != nil && r.frame.owner == m && r.invocation != 0 && r.invocation == r.frame.invocation && r.frame.actual()
}
func (m *UPolynomialManager) displayReceiptPending(r *upolyDisplayInvocationReceipt) bool {
	return m.matchesDisplayReceipt(r) && r.frame.active
}
func (m *UPolynomialManager) displayReceiptSourceCause(r *upolyDisplayInvocationReceipt) error {
	if m.matchesDisplayReceipt(r) {
		return r.frame.sourceCause
	}
	return nil
}
func (m *UPolynomialManager) cleanupDisplayReceipt(r *upolyDisplayInvocationReceipt) error {
	if !m.matchesDisplayReceipt(r) {
		return nil
	}
	return m.cleanupDisplayFrame(r.frame)
}
