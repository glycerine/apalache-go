package polynomial

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Actual raw vector push(default): construct then publish size, with no
// numeral copy. Source callers perform their following Set separately.
func (m *UPolynomialManager) pushDefault(s *upolyVectorState) (*unumeral.Int, error) {
	if s.size == math.MaxUint32 {
		return nil, arena.ErrBounds
	}
	i := s.size
	if e := m.reserve(s, i+1); e != nil {
		return nil, e
	}
	return &m.values(s)[i], nil
}
func (m *UPolynomialManager) flipSign(s *upolyVectorState) error {
	if s.size == 0 {
		return nil
	}
	negative, e := m.modular.IsNeg(&m.values(s)[s.size-1])
	if e != nil {
		return e
	}
	if negative {
		return m.NegViewInPlace(m.stateView(s))
	}
	return nil
}
func (m *UPolynomialManager) mulScalarState(s *upolyVectorState, b *unumeral.Int) error {
	return m.mulScalarStateWithHensel(s, b, nil)
}
func (m *UPolynomialManager) mulScalarStateWithHensel(s *upolyVectorState, b *unumeral.Int, receipt *upolyHenselPairInvocationReceipt) error {
	zero, e := m.modular.IsZero(b)
	if e != nil {
		return e
	}
	if zero {
		return m.reset(s)
	}
	return m.MulScalarViewWithHensel(m.stateView(s), b, receipt)
}
func (m *UPolynomialManager) mulScalarStateWithSourceContext(s *upolyVectorState, b *unumeral.Int, context integerAllocationInvocationContext) error {
	zero, e := m.modular.IsZero(b)
	if e != nil {
		return e
	}
	if zero {
		return m.reset(s)
	}
	return m.mulScalarViewWithSourceContext(m.stateView(s), b, context)
}
func (m *UPolynomialManager) monic(v UPolynomialView, lc, inv *unumeral.Int) error {
	return m.monicWithSourceContext(v, lc, inv, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) monicWithSourceContext(v UPolynomialView, lc, inv *unumeral.Int, context integerAllocationInvocationContext) error {
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SetInt64(lc, 1); e != nil {
		return e
	}
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SetInt64(inv, 1); e != nil {
		return e
	}
	if v.count == 0 {
		return nil
	}
	top, e := m.read(v, v.count-1)
	if e != nil {
		return e
	}
	one, e := m.modular.IsOne(top)
	if e != nil || one {
		return e
	}
	*lc, *top = *top, *lc
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(inv, lc); e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Inv(inv); e != nil {
		return e
	}
	for i := v.count - 1; i > 0; i-- {
		x, e := m.read(v, i-1)
		if e != nil {
			return e
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(x, inv, x); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) monicLocal(v UPolynomialView) error {
	return m.monicLocalWithSourceContext(v, integerAllocationInvocationContext{})
}

// Source: src/math/polynomial/upolynomial.h:361-364.
func (m *UPolynomialManager) monicLocalWithSourceContext(v UPolynomialView, context integerAllocationInvocationContext) (cause error) {
	if n := context.berlekamp; n != nil {
		if !n.NumericBound(true) || n.owner.m != m || context.berlekampGeneration != n.generation {
			return ErrUPolynomialOwner
		}
		local := &n.monic
		if local.declared && local.sourceIndex < 2 && !local.ownEnded {
			return ErrUPolynomialCleanupPending
		}
		*local = upolyBerlekampMonic{declared: true}
		if e := m.monicWithSourceContext(v, &local.lc, &local.inv, context); e != nil {
			return e
		}
		return n.finishMonicEnd()
	}
	owner := context.factorFrontendFrame(m.config.Integer)
	if owner == nil {
		var lc, inv unumeral.Int
		defer func() { m.modular.Del(&lc); m.modular.Del(&inv) }()
		return m.monicWithSourceContext(v, &lc, &inv, context)
	}
	q := &owner.monic
	if q.active || q.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*q = upolyFactorMonicScope{owner: owner, invocation: q.invocation + 1, active: true, executing: true}
	defer func() {
		q.executing = false
		if q.bodyCause == nil {
			q.bodyCause = cause
		}
	}()
	if e := m.monicWithSourceContext(v, &q.lc, &q.inv, context); e != nil {
		return e
	}
	q.endEntered = true
	// These are explicit ascending end DELs, not automatic lexical destruction.
	return q.finishEnd()
}
func (m *UPolynomialManager) MakeMonicView(v UPolynomialView) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.monicLocal(v)
}
func (m *UPolynomialManager) MakeMonic(p UPolynomial) error {
	v, e := m.wholeView(p)
	if e != nil {
		return e
	}
	return m.MakeMonicView(v)
}
func (m *UPolynomialManager) MakeMonicWithLeadingView(v UPolynomialView, lc *unumeral.Int) error {
	if e := m.check(); e != nil {
		return e
	}
	var inv unumeral.Int
	defer m.modular.Del(&inv)
	return m.monic(v, lc, &inv)
}
func (m *UPolynomialManager) MakeMonicWithLeading(p UPolynomial, lc *unumeral.Int) error {
	v, e := m.wholeView(p)
	if e != nil {
		return e
	}
	return m.MakeMonicWithLeadingView(v, lc)
}
func (m *UPolynomialManager) MakeMonicWithLeadingInverseView(v UPolynomialView, lc, inv *unumeral.Int) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.monic(v, lc, inv)
}
func (m *UPolynomialManager) MakeMonicWithLeadingInverse(p UPolynomial, lc, inv *unumeral.Int) error {
	v, e := m.wholeView(p)
	if e != nil {
		return e
	}
	return m.MakeMonicWithLeadingInverseView(v, lc, inv)
}
func (m *UPolynomialManager) gcdEmpty(a, b UPolynomialView, out *upolyVectorState, line int32) (bool, error) {
	for i, input := range [2]UPolynomialView{a, b} {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return true, e
		}
		if enabled && m.alias(input, out) {
			expression := "!is_alias(p1, buffer)"
			if i == 1 {
				expression = "!is_alias(p2, buffer)"
			}
			if e := m.scalarDivisionAssertionFailure(line, expression); e != nil {
				return true, e
			}
		}
	}
	if a.count == 0 {
		if e := m.setView(out, b); e != nil {
			return true, e
		}
		return true, m.flipSign(out)
	}
	if b.count == 0 {
		if e := m.setView(out, a); e != nil {
			return true, e
		}
		return true, m.flipSign(out)
	}
	return false, nil
}

// The retained frontend owns the concrete vector-normalization scope. Other
// callers retain their existing normalization path until their own scope is
// implemented; a frontend scope must never be fabricated for another owner.
func (m *UPolynomialManager) normalizeEuclidWithSourceContext(v UPolynomialView, context integerAllocationInvocationContext) error {
	if context.factorFrontend != nil {
		return m.normalizeWithSourceContext(v, context)
	}
	if n := context.berlekamp; n != nil {
		if !n.NumericBound(true) || n.owner.m != m || context.berlekampGeneration != n.generation {
			return ErrUPolynomialOwner
		}
		return m.normalizeDefaultViewWithReceipt(v, &n.defaultNormalization)
	}
	return m.normalize(v)
}
func (m *UPolynomialManager) euclidGCD(a, b UPolynomialView, out *upolyVectorState) error {
	return m.euclidGCDWithSourceContext(a, b, out, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) euclidGCDWithSourceContext(a, b UPolynomialView, out *upolyVectorState, context integerAllocationInvocationContext) error {
	done, e := m.gcdEmpty(a, b, out, 816)
	if done {
		return e
	}
	field, e := m.modular.Field()
	if e != nil {
		return e
	}
	A, B := &m.gcd1, &m.gcd2
	if e = m.setViewWithSourceContext(A, a, context); e != nil {
		return e
	}
	if e = m.setViewWithSourceContext(B, b, context); e != nil {
		return e
	}
	if e = m.traceEuclidPointers(a, b, B); e != nil {
		return e
	}
	for {
		if e = m.traceEuclidPolynomials(836, a, b, out, context); e != nil {
			return e
		}
		if B.size == 0 {
			if e = m.normalizeEuclidWithSourceContext(m.stateView(A), context); e != nil {
				return e
			}
			*out, *A = *A, *out
			if field {
				if e = m.monicLocalWithSourceContext(m.stateView(out), context); e != nil {
					return e
				}
			} else if e = m.flipSign(out); e != nil {
				return e
			}
			return m.traceEuclidPolynomials(847, a, b, out, context)
		}
		var d uint32
		if e = m.remStateWithSourceContext(m.stateView(A), m.stateView(B), &d, out, context); e != nil {
			return e
		}
		if e = m.normalizeEuclidWithSourceContext(m.stateView(out), context); e != nil {
			return e
		}
		*A, *B = *B, *A
		*B, *out = *out, *B
	}
}
func (m *UPolynomialManager) EuclidGCDViews(a, b UPolynomialView, g UPolynomial) error {
	out, e := m.descriptor(g)
	if e != nil {
		return e
	}
	return m.euclidGCD(a, b, &out.state)
}
func (m *UPolynomialManager) EuclidGCD(a, b, g UPolynomial) error {
	av, e := m.wholeView(a)
	if e != nil {
		return e
	}
	bv, e := m.wholeView(b)
	if e != nil {
		return e
	}
	return m.EuclidGCDViews(av, bv, g)
}
func (m *UPolynomialManager) subresultantGCD(a, b UPolynomialView, out *upolyVectorState) error {
	done, e := m.gcdEmpty(a, b, out, 877)
	if done {
		return e
	}
	A, B := &m.gcd1, &m.gcd2
	var g, h, aux unumeral.Int
	defer func() { m.modular.Del(&aux); m.modular.Del(&h); m.modular.Del(&g) }()
	if e = m.modular.SetInt32(&g, 1); e != nil {
		return e
	}
	if e = m.modular.SetInt32(&h, 1); e != nil {
		return e
	}
	if e = m.setView(A, a); e != nil {
		return e
	}
	if e = m.setView(B, b); e != nil {
		return e
	}
	if A.size < B.size {
		*A, *B = *B, *A
	}
	for {
		if e = m.subresultantDegreeAssertion(A, B); e != nil {
			return e
		}
		if B.size == 0 {
			if e = m.normalize(m.stateView(A)); e != nil {
				return e
			}
			*out, *A = *A, *out
			field, e := m.modular.Field()
			if e != nil {
				return e
			}
			if field {
				return m.monicLocal(m.stateView(out))
			}
			return m.flipSign(out)
		}
		var d uint32
		if e = m.remState(m.stateView(A), m.stateView(B), &d, out); e != nil {
			return e
		}
		delta := A.size - B.size
		if d < delta+1 {
			if e = m.modular.Power(&m.values(B)[B.size-1], delta+1-d, &aux); e != nil {
				return e
			}
			if e = m.mulScalarState(out, &aux); e != nil {
				return e
			}
		}
		d = delta
		if e = m.modular.Power(&h, d, &aux); e != nil {
			return e
		}
		if e = m.modular.Mul(&g, &aux, &aux); e != nil {
			return e
		}
		if e = m.divScalar(m.stateView(out), &aux); e != nil {
			return e
		}
		*A, *B = *B, *A
		*B, *out = *out, *B
		if A.size == 0 {
			return arena.ErrBounds
		}
		if e = m.modular.Copy(&g, &m.values(A)[A.size-1]); e != nil {
			return e
		}
		if e = m.modular.Power(&g, d, &aux); e != nil {
			return e
		}
		if d == 1 {
			if e = m.modular.Copy(&h, &g); e != nil {
				return e
			}
		} else if d > 1 {
			if e = m.subresultantExponentAssertion(d); e != nil {
				return e
			}
			d--
			if e = m.modular.Power(&h, d, &h); e != nil {
				return e
			}
			if e = m.subresultantDivisibilityAssertion(&h, &aux); e != nil {
				return e
			}
			if e = m.modular.Div(&aux, &h, &h); e != nil {
				return e
			}
		}
	}
}
func (m *UPolynomialManager) SubresultantGCDViews(a, b UPolynomialView, g UPolynomial) error {
	out, e := m.descriptor(g)
	if e != nil {
		return e
	}
	return m.subresultantGCD(a, b, &out.state)
}
func (m *UPolynomialManager) SubresultantGCD(a, b, g UPolynomial) error {
	av, e := m.wholeView(a)
	if e != nil {
		return e
	}
	bv, e := m.wholeView(b)
	if e != nil {
		return e
	}
	return m.SubresultantGCDViews(av, bv, g)
}
