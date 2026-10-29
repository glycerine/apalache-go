package polynomial

// Source: upolynomial.cpp:962-981; retained algebraic factor(false) consumer.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// This adapter records only children issued during one accepted square_free
// call. The source's sqf1/sqf2 and GCD work vectors remain manager members.
type upolySquareFreeCallReceipt struct {
	owner      *UPolynomialManager
	invocation uint64
}
type upolySquareFreeCallScope struct {
	image                                            upolySquareFreeImageScope
	owner                                            *UPolynomialManager
	invocation                                       uint64
	active, executing, owning, parentSourceExecuting bool
	hooks                                            unumeral.NumericInvocationHooks
	numeric                                          unumeral.NumericAllocationRoots
	raw                                              []unumeral.IntegerRawInvocationReceipt
	divides                                          []unumeral.IntegerDividesNumeralInvocationReceipt
	pn                                               unumeral.IntegerNormalizeInvocationReceipt
	normalization                                    upolyDefaultNormalizeReceipt
	modGCDTrace, euclidTrace, exactTrace             upolyDisplayInvocationReceipt
	gcdInvocation, craInvocation                     uint64
	bindings                                         [3]unumeral.NumericCallerBinding
	bound                                            uint8
	bodyCause, sourceStop                            error
}

func (q *upolySquareFreeCallScope) valid() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.squareFreeCall
}
func (q *upolySquareFreeCallScope) sourceReadAllowed() bool {
	return q.valid() && (q.parentSourceExecuting || q.owning) && !q.callback()
}
func (q *upolySquareFreeCallScope) NumericBound(executing bool) bool {
	return q.valid() && q.active && !q.owner.closed && (!executing || (q.executing || q.owning) && !q.callback())
}
func (q *upolySquareFreeCallScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !q.NumericBound(true) || q.owner.config.Integer != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &q.numeric
}
func (q *upolySquareFreeCallScope) NumericModularOwner(m *unumeral.IntegerModularManager) bool {
	return q.NumericBound(true) && q.owner.modular == m
}
func (q *upolySquareFreeCallScope) NumericNormalize(m *unumeral.IntegerModularManager, a unumeral.Int) (unumeral.Int, error) {
	if !q.NumericModularOwner(m) {
		return a, ErrUPolynomialOwner
	}
	e := m.PNormalizeWithFactorHistory(&a, &q.pn, unumeral.NumericAllocationContext{}.WithCaller(&q.hooks))
	return a, e
}
func (q *upolySquareFreeCallScope) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if !q.NumericBound(true) || q.owner.config.Integer != z {
		return
	}
	if r.Raw != nil {
		q.raw = append(q.raw, *r.Raw)
	}
	if r.Divides != nil {
		q.divides = append(q.divides, *r.Divides)
	}
}
func (c integerAllocationInvocationContext) squareFreeCallFrame(z *unumeral.IntegerManager) *upolySquareFreeCallScope {
	q := c.squareFreeCall
	if q == nil || !q.NumericBound(true) || q.owner.config.Integer != z || c.squareFreeCallInvocation == 0 || c.squareFreeCallInvocation != q.invocation {
		return nil
	}
	return q
}
func (q *upolySquareFreeCallScope) numericSourceCause() error {
	z := q.owner.config.Integer
	for i := len(q.divides) - 1; i >= 0; i-- {
		if e := z.DividesNumeralInvocationSourceCause(&q.divides[i]); e != nil {
			return e
		}
	}
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := z.RawInvocationSourceCause(&q.raw[i]); e != nil {
			return e
		}
	}
	return nil
}
func (q *upolySquareFreeCallScope) childrenSourceCause() error {
	if e := q.numericSourceCause(); e != nil {
		return e
	}
	if q.image.sourceStop != nil {
		return q.image.sourceStop
	}
	m := q.owner
	for _, r := range []*upolyDisplayInvocationReceipt{&q.exactTrace, &q.euclidTrace, &q.modGCDTrace} {
		if e := r.sourceBlocker(); e != nil {
			return e
		}
	}
	if q.craInvocation != 0 && q.craInvocation == m.craTrace.invocation {
		if e := m.craTrace.sourceFailure(); e != nil {
			return e
		}
	}
	if q.gcdInvocation != 0 && q.gcdInvocation == m.modGCDScalars.invocation {
		if e := m.modGCDScalars.sourceCause; e != nil {
			return e
		}
	}
	if m.defaultNormalizationReceiptPending(&q.normalization) {
		return m.defaultNormalization.sourceCause
	}
	return nil
}
func (q *upolySquareFreeCallScope) pending() bool {
	if !q.valid() || !q.active {
		return false
	}
	m, z := q.owner, q.owner.config.Integer
	if q.image.savedConstructed || q.image.classConstructed || q.image.cConstructed {
		return true
	}
	if q.sourceStop != nil || unumeral.NumericAllocationPending(&q.numeric) || m.modular.MatchesPNormalizeInvocation(&q.pn) || m.defaultNormalizationReceiptPending(&q.normalization) {
		return true
	}
	if q.gcdInvocation != 0 && q.gcdInvocation == m.modGCDScalars.invocation && m.modGCDScalars.ownRemaining != 0 {
		return true
	}
	if q.craInvocation != 0 && q.craInvocation == m.craTrace.invocation && m.craTrace.pending() {
		return true
	}
	for _, r := range []*upolyDisplayInvocationReceipt{&q.exactTrace, &q.euclidTrace, &q.modGCDTrace} {
		if m.displayReceiptPending(r) {
			return true
		}
	}
	for i := range q.raw {
		if z.MatchesRawInvocation(&q.raw[i]) {
			return true
		}
	}
	for i := range q.divides {
		if z.MatchesDividesNumeralInvocation(&q.divides[i]) {
			return true
		}
	}
	return false
}
func (m *UPolynomialManager) squareFreeCallReceiptPending(r *upolySquareFreeCallReceipt) bool {
	return r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.squareFreeCall.invocation && m.squareFreeCall.pending()
}
func (m *UPolynomialManager) squareFreeViewWithReceipt(v UPolynomialView, out UPolynomial, r *upolySquareFreeCallReceipt) (cause error) {
	d, e := m.descriptor(out)
	if e != nil {
		return e
	}
	q := &m.squareFreeCall
	if q.active || q.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*q = upolySquareFreeCallScope{owner: m, invocation: q.invocation + 1, active: true, executing: true, raw: q.raw[:0], divides: q.divides[:0]}
	q.hooks = unumeral.NumericInvocationHooks{Generation: q.invocation, Policy: q}
	if e = q.bind(); e != nil {
		q.active = false
		q.executing = false
		return e
	}
	if r != nil {
		*r = upolySquareFreeCallReceipt{m, q.invocation}
	}
	defer func() {
		q.executing = false
		q.bodyCause = cause
		q.sourceStop = q.childrenSourceCause()
		cause = errors.Join(cause, q.sourceStop)
		if !q.pending() {
			q.active = false
		}
		q.unbind()
	}()
	return m.squareFreeWithContext(v, &d.state, integerAllocationInvocationContext{squareFreeCall: q, squareFreeCallInvocation: q.invocation})
}
func (m *UPolynomialManager) cleanupSquareFreeCall(r *upolySquareFreeCallReceipt) error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.squareFreeCall
	if r != nil && !m.squareFreeCallReceiptPending(r) {
		return nil
	}
	if !q.active {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.owning || q.callback() {
		return ErrUPolynomialCleanupPending
	}
	q.owning = true
	defer func() { q.owning = false }()
	if e := q.bind(); e != nil {
		return e
	}
	defer q.unbind()
	z := m.config.Integer
	if e := q.numeric.Cleanup(z); e != nil {
		return e
	}
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := z.CleanupRawInvocation(&q.raw[i], true); e != nil {
			return e
		}
	}
	for i := len(q.divides) - 1; i >= 0; i-- {
		if e := z.CleanupDividesNumeralInvocation(&q.divides[i], true); e != nil {
			return e
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.pn, true); e != nil {
		return e
	}
	for _, receipt := range []*upolyDisplayInvocationReceipt{&q.exactTrace, &q.euclidTrace, &q.modGCDTrace} {
		if e := receipt.cleanupOwned(); e != nil {
			return e
		}
	}
	if q.craInvocation != 0 && q.craInvocation == m.craTrace.invocation {
		if e := m.craTrace.cleanup(); e != nil {
			return e
		}
	}
	if e := q.image.finish(q, nil, true); e != nil {
		return e
	}
	if q.gcdInvocation != 0 && q.gcdInvocation == m.modGCDScalars.invocation {
		g := &m.modGCDScalars
		for g.ownRemaining > 0 {
			if e := z.CleanupRawNumeral(&g.values[g.ownRemaining-1]); e != nil {
				return e
			}
			g.ownRemaining--
		}
	}
	if e := m.cleanupDefaultNormalizationReceipt(&q.normalization); e != nil {
		return e
	}
	clear(q.raw)
	q.raw = q.raw[:0]
	clear(q.divides)
	q.divides = q.divides[:0]
	q.active = false
	return nil
}

// Actual scoped_set_zp construction and the later scoped c in mod_gcd.
type upolySquareFreeImageScope struct {
	old, c                                                    unumeral.Int
	oldMode, savedConstructed, classConstructed, cConstructed bool
	sourceStop                                                error
}

func (s *upolySquareFreeImageScope) finish(q *upolySquareFreeCallScope, body error, owning bool) error {
	m := q.owner
	z := m.config.Integer
	if !owning {
		if s.sourceStop == nil {
			s.sourceStop = q.numericSourceCause()
		}
		if s.sourceStop == nil {
			s.sourceStop = q.euclidTrace.sourceBlocker()
		}
		if s.sourceStop != nil {
			return errors.Join(body, s.sourceStop)
		}
	}
	del := func(n *unumeral.Int) error {
		if owning {
			return z.CleanupRawNumeral(n)
		}
		_, e := upolynomialSourceNumeralDel(z, n)
		return e
	}
	stop := func(e error) error {
		if !owning && s.sourceStop == nil {
			s.sourceStop = e
		}
		return errors.Join(body, e)
	}
	if s.cConstructed {
		if e := del(&s.c); e != nil {
			return stop(e)
		}
		s.cConstructed = false
	}
	if s.classConstructed {
		var e error
		if s.oldMode {
			e = m.modular.WithAllocationContext(unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)).SetZP(&s.old)
		} else {
			e = m.modular.WithAllocationContext(unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)).SetZ()
		}
		if e != nil {
			return stop(e)
		}
		s.classConstructed = false
	}
	if s.savedConstructed {
		if e := del(&s.old); e != nil {
			return stop(e)
		}
		s.savedConstructed = false
	}
	return body
}

func (q *upolySquareFreeCallScope) callback() bool {
	return q.owner.callbackDepth != 0 || q.owner.config.Integer.RuntimeOwner().FailureTransportActive() || q.owner.modular.BorrowTraceTags().OutputCallbackActive()
}
func (q *upolySquareFreeCallScope) NumericExecuting() bool {
	return q.valid() && q.active && (q.executing || q.owning)
}
func (q *upolySquareFreeCallScope) bind() error {
	z := q.owner.config.Integer
	if e := z.BindCaller(&q.bindings[0], q, 0); e != nil {
		return e
	}
	q.bound++
	if e := z.Naturals().BindCaller(&q.bindings[1], q, 0); e != nil {
		q.unbind()
		return e
	}
	q.bound++
	if e := q.owner.modular.BindCaller(&q.bindings[2], q, 0); e != nil {
		q.unbind()
		return e
	}
	q.bound++
	return nil
}
func (q *upolySquareFreeCallScope) unbind() {
	for q.bound > 0 {
		q.bound--
		if e := q.bindings[q.bound].Unbind(); e != nil {
			panic(e)
		}
	}
}

// CleanupSquareFree acknowledges only this manager's actual accepted call.
func (m *UPolynomialManager) CleanupSquareFree() error { return m.cleanupSquareFreeCall(nil) }

func (q *upolySquareFreeCallScope) NumericCallbackActive() bool { return q.valid() && q.callback() }
