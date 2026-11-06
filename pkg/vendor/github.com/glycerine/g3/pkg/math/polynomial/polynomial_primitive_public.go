package polynomial

// Source: src/math/polynomial/polynomial.cpp:3536-3549,7212-7214;
// src/math/polynomial/polynomial.h; src/util/obj_ref.h.

import (
	"errors"
)

// PrimitivePart preserves the source returned-reference temporary: the helper's
// c/i locals finish before caller assignment, then the returned temporary dies.
// The existing concrete helper implements that same literal lexical sequence.
func (m *MultivariatePolynomialManager) PrimitivePart(p Polynomial, x uint32, result *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	if result == nil || result.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := result.open(); e != nil {
		return e
	}
	return m.gcdPrimitivePart(p, x, &result.root)
}

// IsPrimitive evaluates the source ICCP result equality before destroying the
// pp/c/i locals in reverse declaration order.
func (m *MultivariatePolynomialManager) IsPrimitive(p Polynomial, x uint32) (bool, error) {
	return m.isPrimitiveWithReceipt(p, x, nil)
}
func (m *MultivariatePolynomialManager) isPrimitiveWithReceipt(p Polynomial, x uint32, receipt *polynomialGCDInvocationReceipt) (result bool, err error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return false, e
	}
	if receipt != nil {
		*receipt = polynomialGCDInvocationReceipt{m, s, s.invocation}
	}
	c := m.core
	s.constructNumeral(0)
	s.constructReference(c, 0)
	s.constructReference(c, 1)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	if e = m.iccpPartsWithReceipt(p, x, &s.nums[0], &s.refs[0].root, &s.refs[1].root, &s.iccpInvocation); e != nil {
		return false, e
	}
	return c.Equal(p, s.refs[1].root.value)
}
