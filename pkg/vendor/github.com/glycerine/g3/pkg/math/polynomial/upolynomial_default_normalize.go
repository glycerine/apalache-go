package polynomial

// Source: src/math/polynomial/upolynomial.cpp:366-401.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

// Actual default-call locals, retained only while their source or numerical
// children are incomplete. Direct coefficient calls do not construct a GCD.
type upolyDefaultNormalizeScope struct {
	parentSourceExecuting                      bool
	ownCoefficientRemaining                    uint8
	ownG                                       bool
	owner                                      *UPolynomialManager
	invocation                                 uint64
	active, executing, sourceExecuting, owning bool
	gConstructed                               bool
	coefficientConstructed                     uint8           // old_p_i, then tmp
	values                                     [3]unumeral.Int // g, old_p_i, tmp
	insertions                                 [4]unumeral.NumeralInsertionReceipt
	copy                                       unumeral.IntegerCopyAllocationReceipt
	normalize                                  unumeral.IntegerNormalizeInvocationReceipt
	raw                                        []unumeral.IntegerRawInvocationReceipt
	numeric                                    unumeral.NumericAllocationRoots
	hooks                                      unumeral.NumericInvocationHooks
	borrow                                     upolyFactorTraceBorrow
	bodyCause, sourceCause                     error
}

func (q *upolyDefaultNormalizeScope) sourceReadAllowed() bool {
	return q.valid() && (q.sourceExecuting || q.parentSourceExecuting) && q.owner.callbackDepth == 0 && !q.owner.config.Integer.RuntimeOwner().FailureTransportActive() && !q.owner.modular.BorrowTraceTags().OutputCallbackActive()
}

func (q *upolyDefaultNormalizeScope) valid() bool {
	return q != nil && q.owner != nil && q.owner.self == q.owner && q == &q.owner.defaultNormalization
}
func (q *upolyDefaultNormalizeScope) NumericBound(executing bool) bool {
	return q.valid() && !q.owner.closed && q.active && (!executing || q.executing && !q.sourceExecuting && !q.owning && q.owner.callbackDepth == 0 && !q.owner.config.Integer.RuntimeOwner().FailureTransportActive() && !q.owner.modular.BorrowTraceTags().OutputCallbackActive())
}
func (q *upolyDefaultNormalizeScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !q.NumericBound(true) || z != q.owner.config.Integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &q.numeric
}
func (q *upolyDefaultNormalizeScope) NumericModularOwner(m *unumeral.IntegerModularManager) bool {
	return q.NumericBound(true) && q.owner.modular == m
}
func (q *upolyDefaultNormalizeScope) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if q.NumericBound(true) && z == q.owner.config.Integer && r.Raw != nil {
		q.raw = append(q.raw, *r.Raw)
	}
}
func (q *upolyDefaultNormalizeScope) childSourceCause() error {
	for i := range q.insertions {
		if q.insertions[i].Active() {
			if e := q.insertions[i].SourceCause(); e != nil {
				return e
			}
		}
	}
	z := q.owner.config.Integer
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := z.RawInvocationSourceCause(&q.raw[i]); e != nil {
			return e
		}
	}
	return nil
}
func (q *upolyDefaultNormalizeScope) pending() bool {
	if !q.valid() || !q.active {
		return false
	}
	if q.gConstructed || q.coefficientConstructed != 0 || unumeral.NumericAllocationPending(&q.numeric) {
		return true
	}
	z := q.owner.config.Integer
	if z.MatchesCopyAllocationInvocation(&q.copy) || q.owner.modular.MatchesPNormalizeInvocation(&q.normalize) {
		return true
	}
	for i := range q.raw {
		if z.MatchesRawInvocation(&q.raw[i]) {
			return true
		}
	}
	for i := range q.insertions {
		if q.insertions[i].Active() {
			return true
		}
	}
	return false
}

type upolyDefaultNormalizeReceipt struct {
	owner      *UPolynomialManager
	invocation uint64
}

func (m *UPolynomialManager) defaultNormalizationReceiptPending(r *upolyDefaultNormalizeReceipt) bool {
	return r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.defaultNormalization.invocation && m.defaultNormalization.active
}
func (m *UPolynomialManager) cleanupDefaultNormalizationReceipt(r *upolyDefaultNormalizeReceipt) error {
	if !m.defaultNormalizationReceiptPending(r) {
		return nil
	}
	return m.CleanupNormalization()
}
func (q *upolyDefaultNormalizeScope) NumericNormalize(m *unumeral.IntegerModularManager, a unumeral.Int) (unumeral.Int, error) {
	if !q.NumericModularOwner(m) {
		return a, ErrUPolynomialOwner
	}
	e := m.PNormalizeWithFactorHistory(&a, &q.normalize, unumeral.NumericAllocationContext{}.WithCaller(&q.hooks))
	return a, e
}
func (m *UPolynomialManager) beginDefaultNormalization(g bool) (*upolyDefaultNormalizeScope, error) {
	return m.beginDefaultNormalizationWithReceipt(g, nil)
}
func (m *UPolynomialManager) beginDefaultNormalizationWithReceipt(g bool, receipt *upolyDefaultNormalizeReceipt) (*upolyDefaultNormalizeScope, error) {
	q := &m.defaultNormalization
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	raw := q.raw[:0]
	*q = upolyDefaultNormalizeScope{owner: m, invocation: q.invocation + 1, active: true, executing: true, gConstructed: g, raw: raw}
	q.hooks = unumeral.NumericInvocationHooks{Generation: q.invocation, Policy: q}
	if e := q.borrow.bindManagers([3]*UPolynomialManager{m}, m.modular.BorrowTraceTags()); e != nil {
		q.active = false
		q.executing = false
		return nil, e
	}
	if receipt != nil {
		*receipt = upolyDefaultNormalizeReceipt{m, q.invocation}
	}
	return q, nil
}
func (q *upolyDefaultNormalizeScope) finishCoefficient(body error) error {
	if q.sourceCause == nil {
		q.sourceCause = q.childSourceCause()
	}
	if q.sourceCause == nil {
		q.sourceExecuting = true
		defer func() { q.sourceExecuting = false }()
		for q.coefficientConstructed > 0 {
			if _, e := upolynomialSourceNumeralDel(q.owner.config.Integer, &q.values[q.coefficientConstructed]); e != nil {
				q.sourceCause = e
				break
			}
			q.coefficientConstructed--
		}
	}
	return errors.Join(body, q.sourceCause)
}
func (q *upolyDefaultNormalizeScope) finish(body error) error {
	q.executing = false
	defer q.borrow.unbind()
	q.bodyCause = body
	body = q.finishCoefficient(body)
	if q.sourceCause == nil && q.gConstructed {
		q.sourceExecuting = true
		_, e := upolynomialSourceNumeralDel(q.owner.config.Integer, &q.values[0])
		q.sourceExecuting = false
		if e != nil {
			q.sourceCause = e
		} else {
			q.gConstructed = false
		}
	}
	q.ownCoefficientRemaining = q.coefficientConstructed
	q.ownG = q.gConstructed
	if !q.pending() {
		q.active = false
	}
	return errors.Join(body, q.sourceCause)
}
func (q *upolyDefaultNormalizeScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{defaultNormalization: q, defaultNormalizationInvocation: q.invocation}
}
func (c integerAllocationInvocationContext) defaultNormalizationFrame(z *unumeral.IntegerManager) *upolyDefaultNormalizeScope {
	q := c.defaultNormalization
	if q == nil || !q.NumericBound(true) || q.owner.config.Integer != z || c.defaultNormalizationInvocation == 0 || c.defaultNormalizationInvocation != q.invocation {
		return nil
	}
	return q
}
func (m *UPolynomialManager) normalizeDefaultView(v UPolynomialView) error {
	return m.normalizeDefaultViewWithReceipt(v, nil)
}
func (m *UPolynomialManager) normalizeDefaultViewWithReceipt(v UPolynomialView, receipt *upolyDefaultNormalizeReceipt) (cause error) {
	if v.count <= 1 {
		return m.normalizeWithSourceContext(v, integerAllocationInvocationContext{})
	}
	q, e := m.beginDefaultNormalizationWithReceipt(true, receipt)
	if e != nil {
		return e
	}
	defer func() { cause = q.finish(cause) }()
	g := &q.values[0]
	if e = m.gcdSpanWithSourceContext(v, g, q.context()); e != nil {
		return e
	}
	one, e := m.modular.IsOne(g)
	if e != nil || one {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		if e = q.coefficient(v, i, g); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) normalizeDefaultCoefficient(v UPolynomialView, i uint32, g *unumeral.Int) (cause error) {
	q, e := m.beginDefaultNormalization(false)
	if e != nil {
		return e
	}
	defer func() { cause = q.finish(cause) }()
	return q.coefficient(v, i, g)
}
func (q *upolyDefaultNormalizeScope) coefficient(v UPolynomialView, i uint32, g *unumeral.Int) (cause error) {
	m := q.owner
	old, tmp := &q.values[1], &q.values[2]
	if polynomialSourceDebug {
		*old = unumeral.Int{}
		q.coefficientConstructed = 1
		defer func() { cause = q.finishCoefficient(cause) }()
		x, e := m.read(v, i)
		if e != nil {
			return e
		}
		if e = m.modular.CopyFromOwnerTracked(old, m.config.Integer, x, &q.copy, &q.normalize); e != nil {
			return e
		}
	}
	x, e := m.read(v, i)
	if e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext(q.context().utility(m.config.Integer)).Div(x, g, x); e != nil {
		return e
	}
	if polynomialSourceDebug {
		*tmp = unumeral.Int{}
		q.coefficientConstructed = 2
		if e = m.modular.WithAllocationContext(q.context().utility(m.config.Integer)).Mul(g, x, tmp); e != nil {
			return e
		}
		if e = q.trace(g, x); e != nil {
			return e
		}
		return m.normalizationCoefficientAssert(tmp, old)
	}
	return nil
}
func (q *upolyDefaultNormalizeScope) trace(g, x *unumeral.Int) error {
	if !polynomialSourceDebug {
		return nil
	}
	m := q.owner
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("div_bug") {
		return nil
	}
	if !q.NumericBound(true) || q.coefficientConstructed != 2 {
		return ErrUPolynomialOwner
	}
	var computation error
	_ = tags.EmitConditional("div_bug", "normalize", "src/math/polynomial/upolynomial.cpp", 391, func() bool {
		eq, e := m.modular.Equal(&q.values[2], &q.values[1])
		computation = e
		return e == nil && !eq
	}, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i, a := range [4]*unumeral.Int{&q.values[1], g, x, &q.values[2]} {
			_ = linearWriteText(out, [4]string{"old(p[i]): ", ", g: ", ", p[i]: ", ", tmp: "}[i])
			text, e := m.modular.SourceStringTracked(a, &q.insertions[i])
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

// CleanupNormalization owns the actual default invocation's locals and its
// accepted numerical children. It never scans managers for unrelated work.
func (m *UPolynomialManager) CleanupNormalization() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	q := &m.defaultNormalization
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
	for i := len(q.insertions) - 1; i >= 0; i-- {
		if e := m.modular.CleanupSourceString(&q.insertions[i]); e != nil {
			return e
		}
	}
	if e := q.numeric.Cleanup(m.config.Integer); e != nil {
		return e
	}
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := m.config.Integer.CleanupRawInvocation(&q.raw[i], false); e != nil {
			return e
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
		return e
	}
	if e := m.config.Integer.CleanupCopyAllocationInvocation(&q.copy); e != nil {
		return e
	}
	for q.ownCoefficientRemaining > 0 {
		if e := m.config.Integer.CleanupRawNumeral(&q.values[q.ownCoefficientRemaining]); e != nil {
			return e
		}
		q.ownCoefficientRemaining--
	}
	if q.ownG {
		if e := m.config.Integer.CleanupRawNumeral(&q.values[0]); e != nil {
			return e
		}
		q.ownG = false
	}
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := m.config.Integer.CleanupRawInvocation(&q.raw[i], true); e != nil {
			return e
		}
	}
	q.active = false
	return nil
}
