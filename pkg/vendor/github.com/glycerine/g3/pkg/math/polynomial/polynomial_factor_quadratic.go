package polynomial

// Source: src/math/polynomial/polynomial.cpp:6591-6679;
// src/util/obj_ref.h; src/util/scoped_numeral.h.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) quadraticPending(s *polynomialGCDFunctionScope) bool {
	for i := range s.quadraticArithmetic {
		if m.core.matchesArithmeticInvocation(&s.quadraticArithmetic[i]) {
			return true
		}
	}
	if m.matchesSqrtInvocation(&s.quadraticSqrt) {
		return true
	}
	r := &s.quadraticSqrt
	if r.manager == m && r.frame == &m.squareRoot && r.invocation != 0 {
		for i, a := range m.squareRoot.orphans {
			if a != (Monomial{}) && m.squareRoot.orphanInvocations[i] == r.invocation {
				return true
			}
		}
	}
	for i := range s.quadraticNeg {
		if m.core.matchesNegInvocation(&s.quadraticNeg[i]) || m.core.matchesCoefficientInvocation(&s.quadraticCoefficients[i]) || m.core.matchesFactoryInvocation(&s.quadraticFactory[i]) {
			return true
		}
	}
	return false
}
func (m *MultivariatePolynomialManager) cleanupQuadraticChildren(s *polynomialGCDFunctionScope, abort bool) error {
	for i := len(s.quadraticArithmetic) - 1; i >= 0; i-- {
		if e := m.core.cleanupArithmeticInvocation(&s.quadraticArithmetic[i], abort); e != nil {
			return e
		}
	}
	if e := m.cleanupSqrtInvocation(&s.quadraticSqrt, abort); e != nil {
		return e
	}
	for i := 2; i >= 0; i-- {
		if e := m.core.cleanupNegInvocation(&s.quadraticNeg[i], abort); e != nil {
			return e
		}
		if e := m.core.cleanupFactoryInvocation(&s.quadraticFactory[i], abort); e != nil {
			return e
		}
		if e := m.core.cleanupCoefficientInvocation(&s.quadraticCoefficients[i], abort); e != nil {
			return e
		}
		if e := m.core.cheap.cleanupAppendInvocation(&s.quadraticAppend[i], abort); e != nil {
			return e
		}
	}
	return nil
}

// This is the literal quadratic helper; the enclosing Factor dispatcher and
// univariate backend remain separate source obligations.
func (m *MultivariatePolynomialManager) factorQuadraticSquareFreePrimitive(p Polynomial, r *PolynomialFactors, x, k uint32) error {
	return m.factorQuadraticSquareFreePrimitiveWithReceipts(p, r, x, k, nil, nil, nil)
}
func (m *MultivariatePolynomialManager) factorQuadraticSquareFreePrimitiveWithReceipts(p Polynomial, r *PolynomialFactors, x, k uint32, guards *[2]polynomialGCDInvocationReceipt, body *polynomialGCDInvocationReceipt, phase *uint8) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if r == nil || r.manager != m {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	c := m.core
	if e := polynomialFactorSourceAssert(6601, "degree(p, x) == 2", func() (bool, error) {
		d, e := c.Degree(p, x)
		return d == 2, e
	}); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6602, "is_primitive(p, x)", func() (bool, error) {
		var guard *polynomialGCDInvocationReceipt
		if guards != nil {
			guard = &guards[0]
		}
		if phase != nil {
			*phase = 13
		}
		return m.isPrimitiveWithReceipt(p, x, guard)
	}); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6603, "is_square_free(p, x)", func() (bool, error) {
		var guard *polynomialGCDInvocationReceipt
		if guards != nil {
			guard = &guards[1]
		}
		if phase != nil {
			*phase = 14
		}
		return m.isSquareFreeWithReceipt(p, x, true, guard)
	}); e != nil {
		return e
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	if phase != nil {
		*phase = 12
	}
	if body != nil {
		*body = polynomialGCDInvocationReceipt{m, s, s.invocation}
	}
	defer func() {
		s.bodyCause = cause
		m.gcdDepth--
		cause = errors.Join(cause, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	if e = m.traceQuadraticEntry(p, r, s); e != nil {
		return e
	}
	for i := uint8(0); i < 3; i++ {
		s.constructReference(c, i)
	}
	for i := uint8(0); i < 3; i++ {
		a, e := c.coeffWithReceipts(p, x, 2-uint32(i), &s.quadraticCoefficients[i], &s.quadraticAppend[i], &s.quadraticFactory[i])
		if e != nil {
			return e
		}
		if e = s.refs[i].root.assignWithFactoryReceipt(c, a, &s.quadraticFactory[i]); e != nil {
			return e
		}
	}
	if e = m.traceQuadraticCoefficients(r, s); e != nil {
		return e
	}
	if e = polynomialFactorSourceAssert(6615, "!is_zero(a)", func() (bool, error) {
		zero, e := c.IsZero(s.refs[0].root.value)
		return !zero, e
	}); e != nil {
		return e
	}
	pos, e := c.gradedLexMaxPosition(s.refs[0].root.value)
	if e != nil {
		return e
	}
	if e = polynomialFactorSourceAssert(6617, "a_glex_max_pos != UINT_MAX", func() (bool, error) {
		return pos != ^uint32(0), nil
	}); e != nil {
		return e
	}
	leading, e := c.mutableCoefficientCore(s.refs[0].root.value, pos)
	if e != nil {
		return e
	}
	flipped, e := c.modular.IsNeg(&leading)
	if e != nil {
		return e
	}
	if flipped {
		for i := uint8(0); i < 3; i++ {
			a, e := c.negWithReceipt(s.refs[i].root.value, &s.quadraticNeg[i])
			if e != nil {
				return e
			}
			if e = s.refs[i].root.assign(c, a); e != nil {
				return e
			}
		}
	}
	s.constructReference(c, 3)
	a, e := c.mulWithReceipt(s.refs[1].root.value, s.refs[1].root.value, &s.quadraticArithmetic[0])
	if e != nil {
		return e
	}
	if e = s.refs[3].root.assign(c, a); e != nil {
		return e
	}
	s.constructReference(c, 4)
	a, e = c.mulWithReceipt(s.refs[0].root.value, s.refs[2].root.value, &s.quadraticArithmetic[1])
	if e != nil {
		return e
	}
	if e = s.refs[4].root.assign(c, a); e != nil {
		return e
	}
	s.constructReference(c, 5)
	// m_four and two are source RAW numerals, with no automatic destructor.
	s.nums[8] = unumeral.Int{}
	if e = c.modular.SetInt32(&s.nums[8], -4); e != nil {
		return e
	}
	u, e := c.monomials.Unit()
	if e != nil {
		return e
	}
	one := unumeral.NewInt(1)
	a, e = c.addMulWithReceipt(&one, u, s.refs[3].root.value, &s.nums[8], u, s.refs[4].root.value, &s.quadraticArithmetic[2])
	if e != nil {
		return e
	}
	if e = s.refs[5].root.assign(c, a); e != nil {
		return e
	}
	if e = polynomialFactorSourceAssert(6634, "!is_zero(disc)", func() (bool, error) {
		zero, e := c.IsZero(s.refs[5].root.value)
		return !zero, e
	}); e != nil {
		return e
	}
	s.constructReference(c, 6)
	if e = m.traceQuadraticReference(r, s, 6636, "disc: ", s.refs[5].root.value); e != nil {
		return e
	}
	square, e := m.sqrtWithReceipt(s.refs[5].root.value, &s.refs[6], &s.quadraticSqrt)
	if e != nil {
		return e
	}
	if !square {
		return r.Push(p, k)
	}
	if flipped && k%2 == 1 {
		if e = m.FlipFactorSign(r); e != nil {
			return e
		}
	}
	if polynomialSourceDebug {
		frontier := s.depth
		s.constructReference(c, 11)
		a, e = c.mulWithReceipt(s.refs[6].root.value, s.refs[6].root.value, &s.quadraticArithmetic[3])
		if e != nil {
			return e
		}
		if e = s.refs[11].root.assign(c, a); e != nil {
			return e
		}
		if e = polynomialFactorSourceAssert(6650, "eq(disc, tmp)", func() (bool, error) {
			return c.Equal(s.refs[5].root.value, s.refs[11].root.value)
		}); e != nil {
			return e
		}
		if e = m.finishGCDLocalsTo(s, frontier); e != nil {
			return e
		}
	}
	if e = m.traceQuadraticReference(r, s, 6652, "disc_sqrt: ", s.refs[6].root.value); e != nil {
		return e
	}
	s.nums[9] = unumeral.Int{}
	if e = c.modular.SetInt32(&s.nums[9], 2); e != nil {
		return e
	}
	s.constructReference(c, 7)
	s.constructReference(c, 8)
	s.constructMonomial(0)
	mx, e := c.generatedVariable(x, 1)
	if e != nil {
		return e
	}
	if e = s.monomials[0].assign(c, mx); e != nil {
		return e
	}
	s.constructReference(c, 9)
	a, e = c.scaleWithReceipt(&s.nums[9], s.monomials[0].value, s.refs[0].root.value, &s.quadraticArithmetic[4])
	if e != nil {
		return e
	}
	if e = s.refs[9].root.assign(c, a); e != nil {
		return e
	}
	a, e = c.addWithReceipt(s.refs[9].root.value, s.refs[1].root.value, &s.quadraticArithmetic[5])
	if e != nil {
		return e
	}
	if e = s.refs[7].root.assign(c, a); e != nil {
		return e
	}
	if e = s.refs[8].root.assign(c, s.refs[7].root.value); e != nil {
		return e
	}
	a, e = c.subWithReceipt(s.refs[7].root.value, s.refs[6].root.value, &s.quadraticArithmetic[6])
	if e != nil {
		return e
	}
	if e = s.refs[7].root.assign(c, a); e != nil {
		return e
	}
	a, e = c.addWithReceipt(s.refs[8].root.value, s.refs[6].root.value, &s.quadraticArithmetic[7])
	if e != nil {
		return e
	}
	if e = s.refs[8].root.assign(c, a); e != nil {
		return e
	}
	if e = m.traceQuadraticBeforePrimitive(r, s, x); e != nil {
		return e
	}
	if e = m.gcdPrimitivePart(s.refs[7].root.value, x, &s.refs[7].root); e != nil {
		return e
	}
	if e = m.gcdPrimitivePart(s.refs[8].root.value, x, &s.refs[8].root); e != nil {
		return e
	}
	if e = m.traceQuadraticFactors(r, s); e != nil {
		return e
	}
	if polynomialSourceDebug {
		frontier := s.depth
		s.constructReference(c, 10)
		a, e = c.mulWithReceipt(s.refs[7].root.value, s.refs[8].root.value, &s.quadraticArithmetic[8])
		if e != nil {
			return e
		}
		if e = s.refs[10].root.assign(c, a); e != nil {
			return e
		}
		if flipped {
			a, e = c.negWithReceipt(s.refs[10].root.value, &s.quadraticNeg[2])
			if e != nil {
				return e
			}
			if e = s.refs[10].root.assign(c, a); e != nil {
				return e
			}
		}
		if e = polynomialFactorSourceAssert(6678, "eq(f1f2, p)", func() (bool, error) {
			return c.Equal(s.refs[10].root.value, p)
		}); e != nil {
			return e
		}
		if e = m.finishGCDLocalsTo(s, frontier); e != nil {
			return e
		}
	}
	if e = r.Push(s.refs[7].root.value, k); e != nil {
		return e
	}
	return r.Push(s.refs[8].root.value, k)
}

// The linear source helper constructs no lexical NUM/ref. Its only ordinary
// body action is the existing factors pointer/degree push followed by INC.
func (m *MultivariatePolynomialManager) factorLinearSquareFreePrimitive(p Polynomial, r *PolynomialFactors, x, k uint32) error {
	return m.factorLinearSquareFreePrimitiveWithReceipts(p, r, x, k, nil, nil)
}
func (m *MultivariatePolynomialManager) factorLinearSquareFreePrimitiveWithReceipts(p Polynomial, r *PolynomialFactors, x, k uint32, guards *[2]polynomialGCDInvocationReceipt, phase *uint8) error {
	return m.factorLinearSquareFreePrimitiveWithTraceReceipt(p, r, x, k, guards, phase, nil)
}
func (m *MultivariatePolynomialManager) factorLinearSquareFreePrimitiveWithTraceReceipt(p Polynomial, r *PolynomialFactors, x, k uint32, guards *[2]polynomialGCDInvocationReceipt, phase *uint8, trace *polynomialGCDInvocationReceipt) error {
	if e := m.ready(); e != nil {
		return e
	}
	if r == nil || r.manager != m {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6592, "degree(p, x) == 1", func() (bool, error) {
		d, e := m.core.Degree(p, x)
		return d == 1, e
	}); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6593, "is_primitive(p, x)", func() (bool, error) {
		var guard *polynomialGCDInvocationReceipt
		if guards != nil {
			guard = &guards[0]
		}
		if phase != nil {
			*phase = 13
		}
		return m.isPrimitiveWithReceipt(p, x, guard)
	}); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6594, "is_square_free(p, x)", func() (bool, error) {
		var guard *polynomialGCDInvocationReceipt
		if guards != nil {
			guard = &guards[1]
		}
		if phase != nil {
			*phase = 14
		}
		return m.isSquareFreeWithReceipt(p, x, true, guard)
	}); e != nil {
		return e
	}
	if phase != nil {
		*phase = 12
	}
	if e := m.traceFactorLeaf(p, r, "factor_1_sqf_pp", 6595, "factor square free (degree == 1):\n", trace); e != nil {
		return e
	}
	return r.Push(p, k)
}
