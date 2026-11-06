package polynomial

// Source: src/math/polynomial/polynomial.cpp:6727-6735.

// The source degree-greater-than-two helper currently preserves p as one
// factor. Its source TODO is retained; no alternative factorization is used.
func (m *MultivariatePolynomialManager) factorHigherSquareFreePrimitive(p Polynomial, r *PolynomialFactors, x, k uint32) error {
	return m.factorHigherSquareFreePrimitiveWithReceipts(p, r, x, k, nil, nil)
}
func (m *MultivariatePolynomialManager) factorHigherSquareFreePrimitiveWithReceipts(p Polynomial, r *PolynomialFactors, x, k uint32, guards *[2]polynomialGCDInvocationReceipt, phase *uint8) error {
	return m.factorHigherSquareFreePrimitiveWithTraceReceipt(p, r, x, k, guards, phase, nil)
}
func (m *MultivariatePolynomialManager) factorHigherSquareFreePrimitiveWithTraceReceipt(p Polynomial, r *PolynomialFactors, x, k uint32, guards *[2]polynomialGCDInvocationReceipt, phase *uint8, trace *polynomialGCDInvocationReceipt) error {
	if e := m.ready(); e != nil {
		return e
	}
	if r == nil || r.manager != m {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6728, "degree(p, x) > 2", func() (bool, error) {
		degree, e := m.core.Degree(p, x)
		return degree > 2, e
	}); e != nil {
		return e
	}
	if e := polynomialFactorSourceAssert(6729, "is_primitive(p, x)", func() (bool, error) {
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
	if e := polynomialFactorSourceAssert(6730, "is_square_free(p, x)", func() (bool, error) {
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
	if e := m.traceFactorLeaf(p, r, "factor_n_sqf_pp", 6731, "factor square free (degree > 2):\n", trace); e != nil {
		return e
	}
	return r.Push(p, k)
}
