package polynomial

// Source: src/math/polynomial/polynomial.cpp:4018-4051.

import (
	"errors"
)

func (m *MultivariatePolynomialManager) lcGlexZPX(p Polynomial, x uint32) (result Polynomial, err error) {
	c := m.core
	if e := c.som.reset(); e != nil {
		return result, e
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return result, e
	}
	s.constructMonomial(0)
	s.constructMonomial(1)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	max, tmp := &s.monomials[0], &s.monomials[1]
	size, e := c.readWord(p, 8)
	if e != nil {
		return result, e
	}
	for i := uint32(0); i < size; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return result, e
		}
		k, e := c.monomials.DegreeOf(term, x)
		if e != nil {
			return result, e
		}
		if k != 0 {
			a, e := m.gcdZPXMonomial(s, term, x, 0, false)
			if e != nil {
				return result, e
			}
			if e = tmp.assign(c, a); e != nil {
				return result, e
			}
			term = tmp.value
		}
		greater := max.value == (Monomial{})
		if !greater {
			order, e := c.monomials.GradedLex(term, max.value)
			if e != nil {
				return result, e
			}
			greater = order > 0
		}
		if greater {
			if e = c.som.reset(); e != nil {
				return result, e
			}
			if e = c.som.acquireArithmeticInvocation(&s.product); e != nil {
				return result, e
			}
			if e = max.assign(c, term); e != nil {
				return result, e
			}
			a, e := c.coefficientCore(p, i)
			if e != nil {
				return result, e
			}
			mono, e := m.gcdZPXMonomial(s, Monomial{}, x, k, true)
			if e != nil {
				return result, e
			}
			if e = c.som.addWithArithmeticReceipt(&a, mono, &s.product); e != nil {
				return result, e
			}
		} else if max.value == term {
			a, e := c.coefficientCore(p, i)
			if e != nil {
				return result, e
			}
			mono, e := m.gcdZPXMonomial(s, Monomial{}, x, k, true)
			if e != nil {
				return result, e
			}
			if e = c.som.addWithArithmeticReceipt(&a, mono, &s.product); e != nil {
				return result, e
			}
		}
	}
	if e = polynomialGCDZPXSourceAssert(4041, "!m_som_buffer.empty()", func() (bool, error) {
		n, xerr := c.som.ms.size(c)
		return n != 0, xerr
	}); e != nil {
		return result, e
	}
	if err = m.traceLCGlexZPXMax(max.value); err != nil {
		return result, err
	}
	return c.som.mkWithArithmeticReceipt(false, &s.product)
}

// lcGlexZPXInto is the retained source expression
//
//	destination = lc_glex_ZpX(p, x);
//
// The callee's monomial references and SOM invocation finish before the
// destination assignment starts. The assignment records its completed INC as
// incoming before an old-value DEC can invoke a deletion callback.
func (m *MultivariatePolynomialManager) lcGlexZPXInto(p Polynomial, x uint32, destination *polynomialReference) error {
	if destination == nil {
		return ErrPolynomialOwner
	}
	result, e := m.lcGlexZPX(p, x)
	if e != nil {
		return e
	}
	return destination.assign(m.core, result)
}

func (m *MultivariatePolynomialManager) primitiveZPX(p Polynomial, x uint32, out *polynomialReference) (err error) {
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	s.constructNumeral(0)
	s.constructReference(m.core, 0)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		m.core.pscLeave()
	}()
	return m.iccpZPXWithContentReceipt(p, x, &s.nums[0], &s.refs[0].root, out, &s.contentInvocation)
}
