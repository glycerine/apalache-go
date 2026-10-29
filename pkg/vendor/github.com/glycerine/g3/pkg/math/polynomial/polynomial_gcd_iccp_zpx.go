package polynomial

// Source: src/math/polynomial/polynomial.cpp:3867-4015.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) iccpZPX(p Polynomial, x uint32, ci *unumeral.Int, content, pp *polynomialReference) error {
	return m.iccpZPXWithContentReceipt(p, x, ci, content, pp, nil)
}
func (m *MultivariatePolynomialManager) iccpZPXWithContentReceipt(p Polynomial, x uint32, ci *unumeral.Int, content, pp *polynomialReference, contentReceipt *polynomialContentInvocationReceipt) (err error) {
	c := m.core
	if e := polynomialGCDZPXSourceAssert(3877, "m().modular()", c.modular.Modular); e != nil {
		return e
	}
	if err = m.traceICCPZPXEntry(p, x); err != nil {
		return err
	}
	zero, e := c.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		m.traceICCPZPXLiteral(3880, "iccp_ZpX, p is zero\n")
		if e = c.modular.SetInt32(ci, 0); e != nil {
			return e
		}
		if e = content.assign(c, c.unit); e != nil {
			return e
		}
		return pp.assign(c, p)
	}
	constant, e := c.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		m.traceICCPZPXLiteral(3887, "iccp_ZpX, p is constant\n")
		a, e := c.coefficientCore(p, 0)
		if e != nil {
			return e
		}
		if e = c.modular.Copy(ci, &a); e != nil {
			return e
		}
		if e = content.assign(c, c.unit); e != nil {
			return e
		}
		return pp.assign(c, c.unit)
	}
	d, e := c.Degree(p, x)
	if e != nil {
		return e
	}
	if d == 0 {
		m.traceICCPZPXLiteral(3895, "iccp_ZpX, degree(p, x) == 0\n")
		if e = c.integerContentWithReceipt(p, ci, pp, contentReceipt); e != nil {
			return e
		}
		return content.assign(c, c.unit)
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	s.monomialBuffer.construct()
	s.stack[s.depth] = gcdMonomialBufferID
	s.depth++
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	ms := &s.monomialBuffer
	minimum := ^uint32(0)
	size, e := c.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return e
		}
		k, e := c.monomials.DegreeOf(term, x)
		if e != nil {
			return e
		}
		if k == 0 {
			pos, e := c.queryPositions.Get(term)
			if e != nil {
				return e
			}
			if pos == ^uint32(0) {
				if e = ms.push(c, term); e != nil {
					return e
				}
				if e = c.queryPositions.Set(term, 1); e != nil {
					return e
				}
			}
		}
		if k < minimum {
			minimum = k
		}
	}
	if e = polynomialGCDZPXSourceAssert(3917, "min_degree == 0 || no_x_ms.empty()", func() (bool, error) {
		return minimum == 0 || len(ms.values) == 0, nil
	}); e != nil {
		return e
	}
	if minimum > 0 {
		if e = polynomialGCDZPXSourceAssert(3919, "no_x_ms.empty()", func() (bool, error) { return len(ms.values) == 0, nil }); e != nil {
			return e
		}
		m.traceICCPZPXFactor(x, minimum)
		s.constructReference(c, 0)
		s.constructReference(c, 1)
		a, e := c.MkVariable(x, minimum)
		if e != nil {
			return e
		}
		if e = s.refs[0].root.assign(c, a); e != nil {
			return e
		}
		a, e = c.exactDivWithReceipt(p, s.refs[0].root.value, &s.divisionInvocation)
		if e != nil {
			return e
		}
		if e = s.refs[1].root.assign(c, a); e != nil {
			return e
		}
		if e = m.iccpZPXWithContentReceipt(s.refs[1].root.value, x, ci, content, pp, &s.contentInvocation); e != nil {
			return e
		}
		a, e = c.mulWithReceipt(s.refs[0].root.value, content.value, &s.directArithmetic)
		if e != nil {
			return e
		}
		return content.assign(c, a)
	}
	marked, unmarked := uint32(len(ms.values)), uint32(0)
	s.constructMonomial(0)
	tmp := &s.monomials[0]
	for i := uint32(0); i < size; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return e
		}
		k, e := c.monomials.DegreeOf(term, x)
		if e != nil {
			return e
		}
		if k == 0 {
			continue
		}
		a, e := c.generatedRemoveVariable(term, x)
		if e != nil {
			return e
		}
		if e = tmp.assign(c, a); e != nil {
			return e
		}
		if e = polynomialGCDZPXSourceAssert(3942, "tmp_m != m", func() (bool, error) { return tmp.value != term, nil }); e != nil {
			return e
		}
		pos, e := c.queryPositions.Get(tmp.value)
		if e != nil {
			return e
		}
		if pos == 1 {
			unmarked++
			if e = c.queryPositions.Reset(tmp.value); e != nil {
				return e
			}
			if e = polynomialGCDZPXSourceAssert(3946, "m_m2pos.get(tmp_m) == UINT_MAX", func() (bool, error) {
				pos, xerr := c.queryPositions.Get(tmp.value)
				return pos == ^uint32(0), xerr
			}); e != nil {
				return e
			}
		}
	}
	if e = polynomialGCDZPXSourceAssert(3949, "num_unmarked <= num_marked", func() (bool, error) { return unmarked <= marked, nil }); e != nil {
		return e
	}
	if unmarked < marked {
		for i := uint32(0); i < marked; i++ {
			a, e := ms.get(i)
			if e != nil {
				return e
			}
			if e = c.queryPositions.Reset(a); e != nil {
				return e
			}
		}
		m.traceICCPZPXLiteral(3954, "iccp_ZpX, cheap case... invoking ic\n")
		if e = c.integerContentWithReceipt(p, ci, pp, &s.contentInvocation); e != nil {
			return e
		}
		return content.assign(c, c.unit)
	}
	if e = c.integerContentWithReceipt(p, ci, pp, &s.contentInvocation); e != nil {
		return e
	}
	if e = ms.reset(c, false); e != nil {
		return e
	}
	if e = m.iccp.SetOwner(c); e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		term, e := c.monomialCore(pp.value, i)
		if e != nil {
			return e
		}
		k, e := c.monomials.DegreeOf(term, x)
		if e != nil {
			return e
		}
		if k != 0 {
			a, e := c.generatedRemoveVariable(term, x)
			if e != nil {
				return e
			}
			if e = tmp.assign(c, a); e != nil {
				return e
			}
			term = tmp.value
		}
		pos, e := c.queryPositions.Get(term)
		if e != nil {
			return e
		}
		if pos == ^uint32(0) {
			pos = uint32(len(ms.values))
			if e = ms.push(c, term); e != nil {
				return e
			}
			if e = c.queryPositions.Set(term, pos); e != nil {
				return e
			}
		}
		som, e := m.iccp.At(pos)
		if e != nil {
			return e
		}
		a, e := c.mutableCoefficientCore(pp.value, i)
		if e != nil {
			return e
		}
		mono, e := c.generatedVariable(x, k)
		if e != nil {
			return e
		}
		if e = som.Add(&a, mono); e != nil {
			return e
		}
	}
	count := uint32(len(ms.values))
	for i := uint32(0); i < count; i++ {
		a, e := ms.get(i)
		if e != nil {
			return e
		}
		if e = c.queryPositions.Reset(a); e != nil {
			return e
		}
	}
	if e = polynomialGCDZPXSourceAssert(3987, "num_ms > 0", func() (bool, error) { return count > 0, nil }); e != nil {
		return e
	}
	s.constructReference(c, 0)
	s.constructReference(c, 1)
	g, ng := &s.refs[0].root, &s.refs[1].root
	som, e := m.iccp.At(0)
	if e != nil {
		return e
	}
	a, e := som.Mk(false)
	if e != nil {
		return e
	}
	if e = g.assign(c, a); e != nil {
		return e
	}
	for i := uint32(1); i < count; i++ {
		s.constructReference(c, 2)
		som, e = m.iccp.At(i)
		if e != nil {
			return e
		}
		a, e = som.Mk(false)
		if e != nil {
			return e
		}
		if e = s.refs[2].root.assign(c, a); e != nil {
			return e
		}
		if e = polynomialGCDZPXSourceAssert(3995, "is_univariate(a)", func() (bool, error) {
			return c.IsUnivariate(s.refs[2].root.value)
		}); e != nil {
			return e
		}
		if e = m.gcdEuclid(g.value, s.refs[2].root.value, ng); e != nil {
			return e
		}
		if e = g.assign(c, ng.value); e != nil {
			return e
		}
		one, e := c.IsConstant(g.value)
		if e != nil {
			return e
		}
		if e = m.finishGCDLocalsTo(s, s.depth-1); e != nil {
			return e
		}
		if one {
			break
		}
	}
	one, e := c.IsConstant(g.value)
	if e != nil {
		return e
	}
	if !one {
		if e = m.traceICCPZPXContentBug(g.value, pp.value, x, &s.dividesInvocation); e != nil {
			return e
		}
		if e = content.assign(c, g.value); e != nil {
			return e
		}
		a, e = c.exactDivWithReceipt(pp.value, content.value, &s.divisionInvocation)
		if e != nil {
			return e
		}
		if e = pp.assign(c, a); e != nil {
			return e
		}
	} else {
		if e = content.assign(c, c.unit); e != nil {
			return e
		}
	}
	return m.iccp.ResetN(count)
}
