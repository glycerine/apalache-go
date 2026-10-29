package polynomial

// Source: src/math/polynomial/upolynomial.cpp:213-232; src/util/mpz.cpp:845-912.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// gcdSpan translates raw IntegerManager.GCDMany's source traversal at actual
// array reads. Only the final result receives mpzzp's normalized assertion;
// per-pair modular GCD would introduce different checks/actions.

func (m *UPolynomialManager) gcdSpan(v UPolynomialView, out *unumeral.Int) error {
	return m.gcdSpanWithSourceContext(v, out, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) gcdSpanWithSourceContext(v UPolynomialView, out *unumeral.Int, context integerAllocationInvocationContext) error {
	if e := m.modular.Check(); e != nil {
		return e
	}
	z := m.modular.Integers()
	if e := z.Check(out); e != nil {
		return e
	}
	switch v.count {
	case 0:
		if e := z.Reset(out); e != nil {
			return e
		}
	case 1:
		first, e := m.read(v, 0)
		if e != nil {
			return e
		}
		if e = z.WithAllocationContext((context).utility(z)).CopyFromOwnerTracked(out, z, first, nil); e != nil {
			return e
		}
		if e = z.WithAllocationContext((context).utility(z)).Abs(out); e != nil {
			return e
		}
	default:
		first, e := m.read(v, 0)
		if e != nil {
			return e
		}
		second, e := m.read(v, 1)
		if e != nil {
			return e
		}
		if e = z.WithAllocationContext((context).utility(z)).Gcd(first, second, out); e != nil {
			return e
		}
		for i := uint32(2); i < v.count; i++ {
			if unumeral.IsOneInteger(out) {
				break
			}
			next, e := m.read(v, i)
			if e != nil {
				return e
			}
			if e = z.WithAllocationContext((context).utility(z)).Gcd(out, next, out); e != nil {
				return e
			}
		}
	}
	return m.modular.AssertNormalized(out)
}
func (m *UPolynomialManager) primitiveState(v UPolynomialView, pp *upolyVectorState, cont *unumeral.Int) error {
	return m.primitiveStateWithSourceContext(v, pp, cont, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) primitiveStateWithSourceContext(v UPolynomialView, pp *upolyVectorState, cont *unumeral.Int, context integerAllocationInvocationContext) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && v.count == 0 {
		if e := m.scalarDivisionAssertionFailure(214, "f_sz > 0"); e != nil {
			return e
		}
	}
	if e := m.gcdSpanWithSourceContext(v, cont, context); e != nil {
		return e
	}
	enabled, e = refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		positive, e := m.modular.IsPos(cont)
		if e != nil {
			return e
		}
		if !positive {
			if e := m.scalarDivisionAssertionFailure(216, "m().is_pos(cont)"); e != nil {
				return e
			}
		}
	}
	one, e := m.modular.IsOne(cont)
	if e != nil {
		return e
	}
	if one {
		return m.setViewWithSourceContext(pp, v, context)
	}
	if e = m.reserve(pp, v.count); e != nil {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		x, e := m.read(v, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(x)
		if e != nil {
			return e
		}
		dst := &m.values(pp)[i]
		if zero {
			e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SetInt64(dst, 0)
		} else {
			e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Div(x, cont, dst)
		}
		if e != nil {
			return e
		}
	}
	return m.setSize(pp, v.count)
}
func (m *UPolynomialManager) PrimitiveAndContentViews(v UPolynomialView, pp UPolynomial, cont *unumeral.Int) error {
	out, e := m.descriptor(pp)
	if e != nil {
		return e
	}
	return m.primitiveState(v, &out.state, cont)
}
func (m *UPolynomialManager) PrimitiveAndContent(p, pp UPolynomial, cont *unumeral.Int) error {
	v, e := m.wholeView(p)
	if e != nil {
		return e
	}
	return m.PrimitiveAndContentViews(v, pp, cont)
}
func (m *UPolynomialManager) primitive(v UPolynomialView, pp *upolyVectorState) error {
	return m.primitiveWithSourceContext(v, pp, integerAllocationInvocationContext{})
}

// Source: src/math/polynomial/upolynomial.h:212-215.
func (m *UPolynomialManager) primitiveWithSourceContext(v UPolynomialView, pp *upolyVectorState, context integerAllocationInvocationContext) (cause error) {
	owner := context.factorFrontendFrame(m.config.Integer)
	if owner == nil {
		var cont unumeral.Int
		defer m.modular.Del(&cont)
		return m.primitiveStateWithSourceContext(v, pp, &cont, context)
	}
	q := &owner.primitive
	if q.constructed || q.executing || q.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*q = upolyFactorPrimitiveScope{owner: owner, invocation: q.invocation + 1, constructed: true, executing: true}
	defer func() { q.executing = false; cause = q.finish(cause) }()
	return m.primitiveStateWithSourceContext(v, pp, &q.cont, context)
}
func (m *UPolynomialManager) PrimitiveView(v UPolynomialView, pp UPolynomial) error {
	out, e := m.descriptor(pp)
	if e != nil {
		return e
	}
	return m.primitive(v, &out.state)
}
func (m *UPolynomialManager) Primitive(p, pp UPolynomial) error {
	v, e := m.wholeView(p)
	if e != nil {
		return e
	}
	return m.PrimitiveView(v, pp)
}
func (m *UPolynomialManager) normalize(v UPolynomialView) error {
	return (m).normalizeWithSourceContext(v, integerAllocationInvocationContext{})
}

// Source: src/math/polynomial/upolynomial.cpp:366-401.
func (m *UPolynomialManager) normalizeWithSourceContext(v UPolynomialView, context integerAllocationInvocationContext) (cause error) {
	if context.squareFreeCall != nil {
		q := context.squareFreeCallFrame(m.config.Integer)
		if q == nil {
			return ErrUPolynomialOwner
		}
		return m.normalizeDefaultViewWithReceipt(v, &q.normalization)
	}
	if context.sturmIsolation != nil {
		q := context.sturmIsolationFrame(m.config.Integer)
		if q == nil {
			return ErrUPolynomialOwner
		}
		if v.count > 1 {
			return q.normalizeView(v)
		}
	}

	if context.sequenceSturm != nil {
		q := context.sequenceSturmFrame(m.config.Integer)
		if q == nil {
			return ErrUPolynomialOwner
		}
		if v.count > 1 {
			return q.normalizeView(v)
		}
	}
	if context.rootID != nil {
		q := context.rootIDFrame(m.config.Integer)
		if q == nil {
			return ErrUPolynomialOwner
		}
		if v.count > 1 {
			return q.normalizeView(v)
		}
	}
	if v.count == 0 {
		return nil
	}
	if v.count == 1 {
		x, e := m.read(v, 0)
		if e != nil {
			return e
		}
		positive, e := m.modular.IsPos(x)
		if e != nil {
			return e
		}
		if positive {
			return m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SetInt64(x, 1)
		}
		return m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SetInt64(x, -1)
	}
	if context == (integerAllocationInvocationContext{}) {
		return m.normalizeDefaultView(v)
	}
	o := context.factorFrontendFrame(m.config.Integer)
	if o == nil || !o.executing || o.service.manager != m {
		return ErrUPolynomialOwner
	}
	q := &o.vectorNormalize
	if q.constructed != 0 || q.executing || q.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*q = upolyFactorVectorNormalizeScope{owner: o, invocation: q.invocation + 1, constructed: 1, peak: 1, executing: true}
	g := &q.g
	defer func() { q.executing = false; cause = q.finish(cause) }()
	if e := m.gcdSpanWithSourceContext(v, g, context); e != nil {
		return e
	}
	one, e := m.modular.IsOne(g)
	if e != nil || one {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		if e = m.normalizeCoefficientWithSourceContext(v, i, g, q, context); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) normalizeCoefficient(v UPolynomialView, i uint32, g *unumeral.Int) error {
	return m.normalizeCoefficientWithSourceContext(v, i, g, nil, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) normalizeCoefficientWithSourceContext(v UPolynomialView, i uint32, g *unumeral.Int, q *upolyFactorVectorNormalizeScope, context integerAllocationInvocationContext) (cause error) {
	if q == nil && context == (integerAllocationInvocationContext{}) {
		return m.normalizeDefaultCoefficient(v, i, g)
	}
	if q == nil {
		return ErrUPolynomialOwner
	}
	old, tmp := &q.old, &q.tmp
	// Source Z3DEBUG lexical temporaries are destroyed tmp then old.
	if polynomialSourceDebug {
		if !q.valid() || !q.executing || q.constructed != 1 {
			return ErrUPolynomialOwner
		}
		q.old = unumeral.Int{}
		q.constructed = 2
		q.peak = 2
		q.coefficientExecuting = true
		defer func() { q.coefficientExecuting = false; cause = q.finishCoefficient(cause) }()
		x, e := m.read(v, i)
		if e != nil {
			return e
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(old, x); e != nil {
			return e
		}
	}
	x, e := m.read(v, i)
	if e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Div(x, g, x); e != nil {
		return e
	}
	if polynomialSourceDebug {
		q.tmp = unumeral.Int{}
		q.constructed = 3
		q.peak = 3
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(g, x, tmp); e != nil {
			return e
		}
		if e = m.traceFrontendNormalization(q.owner, old, g, x, tmp); e != nil {
			return e
		}
		return m.normalizationCoefficientAssert(tmp, old)
	}
	return nil
}
func (m *UPolynomialManager) NormalizeView(v UPolynomialView) error {
	return m.normalizeViewWithReceipt(v, nil)
}
func (m *UPolynomialManager) normalizeViewWithReceipt(v UPolynomialView, receipt *upolyDefaultNormalizeReceipt) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.normalizeDefaultViewWithReceipt(v, receipt)
}
func (m *UPolynomialManager) Normalize(p UPolynomial) error {
	v, e := m.wholeView(p)
	if e != nil {
		return e
	}
	return m.NormalizeView(v)
}

// WithCount is the explicit source (pointer,unsigned size) boundary: it
// changes only the scalar count. It neither borrows/dereferences an owner nor
// prevalidates physical extent. Algorithms guard at each actual source read;
// an early return may therefore skip an unavailable tail exactly as C++ does.
func (v UPolynomialView) WithCount(count uint32) UPolynomialView {
	v.count = count
	return v
}
