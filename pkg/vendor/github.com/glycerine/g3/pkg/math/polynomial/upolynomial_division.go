package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/math/polynomial/upolynomial.cpp:403-651; outputs keep completed prefixes.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *UPolynomialManager) divScalar(v UPolynomialView, b *unumeral.Int) error {
	return m.divScalarWithHensel(v, b, nil)
}
func (m *UPolynomialManager) divScalarWithHensel(v UPolynomialView, b *unumeral.Int, receipt *upolyHenselPairInvocationReceipt) error {
	context := numericIntegerModularManager_henselPairAllocationContext(m.modular, receipt)
	if receipt != nil && context.henselPairReceipt(m.config.Integer) == nil {
		return ErrUPolynomialOwner
	}
	return m.divScalarWithSourceContext(v, b, context)
}
func (m *UPolynomialManager) divScalarWithSourceContext(v UPolynomialView, b *unumeral.Int, context integerAllocationInvocationContext) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		zero, e := m.modular.IsZero(b)
		if e != nil {
			return e
		}
		if zero {
			if e := m.scalarDivisionAssertionFailure(404, "!m().is_zero(b)"); e != nil {
				return e
			}
		}
	}
	one, e := m.modular.IsOne(b)
	if e != nil || one {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		x, e := m.read(v, i)
		if e != nil {
			return e
		}
		enabled, e = refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled {
			ok, e := numericIntegerModularManager_dividesWithHenselContext(m.modular, b, x, context)
			if e != nil {
				return e
			}
			if !ok {
				if e := m.scalarDivisionAssertionFailure(409, "m().divides(b, p[i])"); e != nil {
					return e
				}
			}
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Div(x, b, x); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) DivScalar(p UPolynomial, b *unumeral.Int) error {
	v, e := m.wholeView(p)
	if e != nil {
		return e
	}
	return m.divScalar(v, b)
}
func (m *UPolynomialManager) DivScalarView(v UPolynomialView, b *unumeral.Int) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.divScalar(v, b)
}

// Concrete borrowed divisor representation; immediate is used only for the
// actual source lexical mpz[-1,2]. Pointer receivers borrow that caller frame,
// never an address of a by-value numeral copy or a fake directory binding.
type upolyDivisorSpan struct {
	view      UPolynomialView
	immediate bool
	pair      [2]unumeral.Int
}

func (b *upolyDivisorSpan) count() uint32 {
	if b.immediate {
		return 2
	}
	return b.view.count
}
func (m *UPolynomialManager) readDivisor(b *upolyDivisorSpan, i uint32) (*unumeral.Int, error) {
	if b.immediate {
		if i >= 2 {
			return nil, arena.ErrBounds
		}
		return &b.pair[i], nil
	}
	return m.read(b.view, i)
}
func (m *UPolynomialManager) aliasDivisor(b *upolyDivisorSpan, s *upolyVectorState) bool {
	return !b.immediate && m.alias(b.view, s)
}
func (m *UPolynomialManager) divRemCore(a, b UPolynomialView, d *uint32, q, r *upolyVectorState) error {
	span := upolyDivisorSpan{view: b}
	return m.divRemCoreSpan(a, &span, d, q, r)
}
func (m *UPolynomialManager) divRemCoreSpan(a UPolynomialView, b *upolyDivisorSpan, d *uint32, q, r *upolyVectorState) error {
	return m.divRemCoreSpanWithHensel(a, b, d, q, r, nil)
}
func (m *UPolynomialManager) divRemCoreWithHensel(a, b UPolynomialView, d *uint32, q, r *upolyVectorState, receipt *upolyHenselPairInvocationReceipt) error {
	span := upolyDivisorSpan{view: b}
	return m.divRemCoreSpanWithHensel(a, &span, d, q, r, receipt)
}
func (m *UPolynomialManager) divRemCoreSpanWithHensel(a UPolynomialView, b *upolyDivisorSpan, d *uint32, q, r *upolyVectorState, receipt *upolyHenselPairInvocationReceipt) (failure error) {
	context := numericIntegerModularManager_henselPairAllocationContext(m.modular, receipt)
	if receipt != nil && context.henselPairReceipt(m.config.Integer) == nil {
		return ErrUPolynomialOwner
	}
	return m.divRemCoreSpanWithSourceContext(a, b, d, q, r, receipt, nil, context)
}
func (m *UPolynomialManager) divRemCoreSpanWithSourceContext(a UPolynomialView, b *upolyDivisorSpan, d *uint32, q, r *upolyVectorState, receipt *upolyHenselPairInvocationReceipt, caller *upolyHenselCallerScope, context integerAllocationInvocationContext) (failure error) {
	if context.removeOneHalfRoot != nil {
		frame := context.removeOneHalfRootFrame(m.config.Integer)
		if frame == nil || frame.owner != m || frame.owner.modular != m.modular {
			return ErrUPolynomialOwner
		}
	}
	if d == nil {
		return arena.ErrBounds
	}
	if e := m.divRemAliasAssertions(a, b, q, r); e != nil {
		return e
	}
	*d = 0
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && b.count() == 0 {
		if e := m.scalarDivisionAssertionFailure(437, "sz2 > 0"); e != nil {
			return e
		}
	}
	if b.count() == 1 {
		if e := m.setViewWithSourceContext(q, a, context); e != nil {
			return e
		}
		field, e := m.modular.Field()
		if e != nil {
			return e
		}
		if field {
			bn, e := m.readDivisor(b, 0)
			if e != nil {
				return e
			}
			if e = m.divScalarWithSourceContext(m.stateView(q), bn, context); e != nil {
				return e
			}
		}
		return m.reset(r)
	}
	if e := m.reset(q); e != nil {
		return e
	}
	if e := m.setViewWithSourceContext(r, a, context); e != nil {
		return e
	}
	if a.count <= 1 {
		return nil
	}
	var qsz uint32
	if a.count >= b.count() {
		qsz = a.count - b.count() + 1
		if e := m.reserve(q, qsz); e != nil {
			return e
		}
	}
	bn, e := m.readDivisor(b, b.count()-1)
	if e != nil {
		return e
	}
	enabled, e = refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		zero, e := m.modular.IsZero(bn)
		if e != nil {
			return e
		}
		if zero {
			if e := m.scalarDivisionAssertionFailure(459, "!m().is_zero(b_n)"); e != nil {
				return e
			}
		}
	}
	var local unumeral.Int
	am := &local
	if receipt != nil {
		h := receipt.frame
		h.am = unumeral.Int{}
		h.amConstructed = true
		am = &h.am
	}
	// The selected Berlekamp/Remove/Hensel callers retain this actual lexical
	// destructor. Legacy nil public Div/DivRem still ignore Del errors and remain
	// a separate caller-lifetime obligation; scratch/source prefixes stay published.
	if caller != nil {
		caller.am = unumeral.Int{}
		caller.amConstructed = true
		caller.amEnded = false
		caller.amOwned = false
		am = &caller.am
	}
	if n := context.berlekamp; n != nil {
		if !n.NumericBound(true) || n.owner.m != m || context.berlekampGeneration != n.generation {
			return ErrUPolynomialOwner
		}
		local := &n.divisionAM
		if local.declared && !local.sourceEnded && !local.ownEnded {
			return ErrUPolynomialCleanupPending
		}
		*local = upolyBerlekampScopedNumeral{declared: true}
		am = &local.value
		defer func() { failure = n.owner.finishScopedNumeral(local, failure) }()
	} else if removing := context.removeOneHalfRootFrame(m.config.Integer); removing != nil {
		removing.values[0] = unumeral.Int{}
		removing.constructed = 1
		removing.sourceRemaining = 1
		am = &removing.values[0]
		defer func() { failure = removing.finishAmSource(failure) }()
	} else if receipt == nil && caller == nil {
		defer m.modular.Del(am)
	} else if caller != nil {
		defer func() { failure = joinHenselCaller(failure, caller.finishAmSource(failure)) }()
	} else {
		defer func() { failure = receipt.service.finishHenselAmSource(receipt, failure) }()
	}
	for {
		if e = m.checkpoint(); e != nil {
			return e
		}
		n := r.size
		if n < b.count() {
			return m.setSize(q, qsz)
		}
		delta := n - b.count()
		field, e := m.modular.Field()
		if e != nil {
			return e
		}
		bn, e = m.readDivisor(b, b.count()-1)
		if e != nil {
			return e
		}
		if field {
			if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Div(&m.values(r)[n-1], bn, am); e != nil {
				return e
			}
			if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Add(&m.values(q)[delta], am, &m.values(q)[delta]); e != nil {
				return e
			}
		} else {
			*d++
			if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(am, &m.values(r)[n-1]); e != nil {
				return e
			}
			for i := uint32(0); i < n-1; i++ {
				bn, e = m.readDivisor(b, b.count()-1)
				if e != nil {
					return e
				}
				if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(&m.values(r)[i], bn, &m.values(r)[i]); e != nil {
					return e
				}
			}
			for i := uint32(0); i < qsz; i++ {
				bn, e = m.readDivisor(b, b.count()-1)
				if e != nil {
					return e
				}
				if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(&m.values(q)[i], bn, &m.values(q)[i]); e != nil {
					return e
				}
			}
			if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Add(&m.values(q)[delta], am, &m.values(q)[delta]); e != nil {
				return e
			}
		}
		for i := uint32(0); i < b.count()-1; i++ {
			x, e := m.readDivisor(b, i)
			if e != nil {
				return e
			}
			dst := &m.values(r)[i+delta]
			if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SubMul(dst, am, x, dst, removeOrHenselAddMulChild(context, receipt, caller)); e != nil {
				return e
			}
		}
		if e = m.setSize(r, n-1); e != nil {
			return e
		}
	}
}
func (m *UPolynomialManager) DivRemViews(a, b UPolynomialView, q, r UPolynomial, d *uint32) error {
	qo, e := m.descriptor(q)
	if e != nil {
		return e
	}
	ro, e := m.descriptor(r)
	if e != nil {
		return e
	}
	if e = m.divRemCore(a, b, d, &m.div2, &m.div1); e != nil {
		return e
	}
	ro.state, m.div1 = m.div1, ro.state
	qo.state, m.div2 = m.div2, qo.state
	return nil
}
func (m *UPolynomialManager) DivRem(a, b, q, r UPolynomial, d *uint32) error {
	av, e := m.wholeView(a)
	if e != nil {
		return e
	}
	bv, e := m.wholeView(b)
	if e != nil {
		return e
	}
	return m.DivRemViews(av, bv, q, r, d)
}
func (m *UPolynomialManager) divState(a, b UPolynomialView, q *upolyVectorState) error {
	return m.divStateWithSourceContext(a, b, q, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) divStateWithSourceContext(a, b UPolynomialView, q *upolyVectorState, context integerAllocationInvocationContext) error {
	span := upolyDivisorSpan{view: b}
	return m.divSpanStateWithSourceContext(a, &span, q, context)
}
func (m *UPolynomialManager) divSpanState(a UPolynomialView, b *upolyDivisorSpan, q *upolyVectorState) error {
	return m.divSpanStateWithSourceContext(a, b, q, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) divSpanStateWithSourceContext(a UPolynomialView, b *upolyDivisorSpan, q *upolyVectorState, context integerAllocationInvocationContext) error {
	var d uint32
	if e := m.divRemCoreSpanWithSourceContext(a, b, &d, &m.div2, &m.div1, nil, nil, context); e != nil {
		return e
	}
	if e := m.reset(&m.div1); e != nil {
		return e
	}
	*q, m.div2 = m.div2, *q
	return nil
}
func (m *UPolynomialManager) DivViews(a, b UPolynomialView, q UPolynomial) error {
	out, e := m.descriptor(q)
	if e != nil {
		return e
	}
	return m.divState(a, b, &out.state)
}
func (m *UPolynomialManager) Div(a, b, q UPolynomial) error {
	av, e := m.wholeView(a)
	if e != nil {
		return e
	}
	bv, e := m.wholeView(b)
	if e != nil {
		return e
	}
	return m.DivViews(av, bv, q)
}

func (m *UPolynomialManager) dividesViews(a, b UPolynomialView) (bool, error) {
	return m.dividesViewsWithSourceContext(a, b, integerAllocationInvocationContext{})
}

// Source: src/math/polynomial/upolynomial.cpp:579-607.
func (m *UPolynomialManager) dividesViewsWithSourceContext(a, b UPolynomialView, context integerAllocationInvocationContext) (answer bool, cause error) {
	if b.count == 0 {
		return false, nil
	}
	if a.count == 0 {
		return true, nil
	}
	if b.count > a.count {
		return false, nil
	}
	bn, e := m.read(b, b.count-1)
	if e != nil {
		return false, e
	}
	an, e := m.read(a, a.count-1)
	if e != nil {
		return false, e
	}
	ok, e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Divides(bn, an)
	if e != nil || !ok {
		return false, e
	}
	var local unumeral.Int
	ratio := &local
	var frame *upolyFactorDivisibilityScope
	if owner := context.factorFrontendFrame(m.config.Integer); owner != nil {
		frame = &owner.divisibility
		if frame.constructed || frame.executing || frame.invocation == ^uint64(0) {
			return false, ErrUPolynomialCleanupPending
		}
		*frame = upolyFactorDivisibilityScope{owner: owner, invocation: frame.invocation + 1, executing: true, constructed: true}
		ratio = &frame.ratio
		defer func() { frame.executing = false; cause = frame.finish(cause) }()
	} else {
		defer m.modular.Del(ratio)
	}
	r := &m.div1
	if e = m.setViewWithSourceContext(r, a, context); e != nil {
		return false, e
	}
	n := a.count
	for {
		if n == 0 {
			return true, nil
		}
		if b.count > n {
			return false, nil
		}
		bn, e = m.read(b, b.count-1)
		if e != nil {
			return false, e
		}
		ok, e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Divides(bn, &m.values(r)[n-1])
		if e != nil || !ok {
			return false, e
		}
		delta := n - b.count
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Div(&m.values(r)[n-1], bn, ratio); e != nil {
			return false, e
		}
		for i := uint32(0); i < b.count-1; i++ {
			x, e := m.read(b, i)
			if e != nil {
				return false, e
			}
			zero, e := m.modular.IsZero(x)
			if e != nil {
				return false, e
			}
			if !zero {
				dst := &m.values(r)[i+delta]
				var child *unumeral.IntegerAddMulInvocationReceipt
				if frame != nil {
					child = &frame.addMul
				}
				if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SubMul(dst, ratio, x, dst, child); e != nil {
					return false, e
				}
			}
		}
		if e = m.modular.Reset(&m.values(r)[n-1]); e != nil {
			return false, e
		}
		if e = m.trim(r); e != nil {
			return false, e
		}
		n = r.size
	}
}
func (m *UPolynomialManager) DividesViews(a, b UPolynomialView) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	return m.dividesViews(a, b)
}
func (m *UPolynomialManager) Divides(a, b UPolynomial) (bool, error) {
	av, e := m.wholeView(a)
	if e != nil {
		return false, e
	}
	bv, e := m.wholeView(b)
	if e != nil {
		return false, e
	}
	return m.DividesViews(av, bv)
}

func (m *UPolynomialManager) exactDivState(a, b UPolynomialView, out *upolyVectorState) (bool, error) {
	return m.exactDivStateWithSourceContext(a, b, out, nil, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) exactDivStateWithSourceContext(a, b UPolynomialView, out *upolyVectorState, child *unumeral.IntegerAddMulInvocationReceipt, context integerAllocationInvocationContext) (bool, error) {
	if m.pendingExactDivTrace != nil && m.pendingExactDivTrace.owner.displayReceiptPending(m.pendingExactDivTrace) {
		return false, ErrUPolynomialCleanupPending
	}
	if b.count == 0 {
		return false, nil
	}
	if a.count == 0 {
		return true, m.reset(out)
	}
	if b.count > a.count {
		return false, nil
	}
	bn, e := m.read(b, b.count-1)
	if e != nil {
		return false, e
	}
	an, e := m.read(a, a.count-1)
	if e != nil {
		return false, e
	}
	ok, e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Divides(bn, an)
	if e != nil || !ok {
		return false, e
	}
	b0, e := m.read(b, 0)
	if e != nil {
		return false, e
	}
	a0, e := m.read(a, 0)
	if e != nil {
		return false, e
	}
	ok, e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Divides(b0, a0)
	if e != nil || !ok {
		return false, e
	}
	q := &m.exactDiv
	r := &m.div1
	if e = m.reset(q); e != nil {
		return false, e
	}
	deg := a.count - b.count
	if e = m.reserve(q, deg+1); e != nil {
		return false, e
	}
	if e = m.traceExactDivPointers(a); e != nil {
		return false, e
	}
	if e = m.setViewWithSourceContext(r, a, context); e != nil {
		return false, e
	}
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return false, e
	}
	if enabled && r.size != a.count {
		if e = m.scalarDivisionAssertionFailure(626, "_p1.size() == sz1"); e != nil {
			return false, e
		}
	}
	n := a.count
	for {
		if e := m.traceExactDivLoop(context); e != nil {
			return false, e
		}
		if n == 0 {
			if e = m.setSize(q, deg+1); e != nil {
				return false, e
			}
			*out, *q = *q, *out
			return true, nil
		}
		reject := b.count > n
		if !reject {
			bn, e = m.read(b, b.count-1)
			if e != nil {
				return false, e
			}
			ok, e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Divides(bn, &m.values(r)[n-1])
			if e != nil {
				return false, e
			}
			reject = !ok
		}
		if !reject {
			b0, e = m.read(b, 0)
			if e != nil {
				return false, e
			}
			ok, e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Divides(b0, &m.values(r)[0])
			if e != nil {
				return false, e
			}
			reject = !ok
		}
		if reject {
			return false, m.reset(out)
		}
		delta := n - b.count
		dst := &m.values(q)[delta]
		bn, e = m.read(b, b.count-1)
		if e != nil {
			return false, e
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Div(&m.values(r)[n-1], bn, dst); e != nil {
			return false, e
		}
		for i := uint32(0); i < b.count-1; i++ {
			x, e := m.read(b, i)
			if e != nil {
				return false, e
			}
			zero, e := m.modular.IsZero(x)
			if e != nil {
				return false, e
			}
			if !zero {
				row := &m.values(r)[i+delta]
				if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SubMul(row, dst, x, row, child); e != nil {
					return false, e
				}
			}
		}
		if e = m.modular.Reset(&m.values(r)[n-1]); e != nil {
			return false, e
		}
		if e = m.trim(r); e != nil {
			return false, e
		}
		n = r.size
	}
}
func (m *UPolynomialManager) ExactDivViews(a, b UPolynomialView, q UPolynomial) (bool, error) {
	out, e := m.descriptor(q)
	if e != nil {
		return false, e
	}
	return m.exactDivState(a, b, &out.state)
}
func (m *UPolynomialManager) ExactDiv(a, b, q UPolynomial) (bool, error) {
	av, e := m.wholeView(a)
	if e != nil {
		return false, e
	}
	bv, e := m.wholeView(b)
	if e != nil {
		return false, e
	}
	return m.ExactDivViews(av, bv, q)
}
