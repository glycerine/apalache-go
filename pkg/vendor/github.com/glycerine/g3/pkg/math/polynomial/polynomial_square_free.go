package polynomial

// Source: src/math/polynomial/polynomial.cpp:4529-4598,7232-7246;
// src/math/polynomial/polynomial.h; src/util/obj_ref.h.

import (
	"errors"
)

func (m *MultivariatePolynomialManager) SquareFree(p Polynomial, out *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	if out == nil || out.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	return m.squareFreeInto(p, &out.root)
}
func (m *MultivariatePolynomialManager) SquareFreeVariable(p Polynomial, x uint32, out *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	if out == nil || out.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	return m.squareFreeVariableInto(p, x, &out.root)
}
func (m *MultivariatePolynomialManager) squareFreeInto(p Polynomial, out *polynomialReference) (err error) {
	c := m.core
	zero, e := c.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		return out.assign(c, c.zero)
	}
	constant, e := c.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		return out.assign(c, p)
	}
	x, e := c.MaxVariable(p)
	if e != nil {
		return e
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	s.constructNumeral(0)
	s.constructReference(c, 0)
	s.constructReference(c, 1)
	if e = m.iccpPartsWithReceipt(p, x, &s.nums[0], &s.refs[0].root, &s.refs[1].root, &s.iccpInvocation); e != nil {
		return e
	}
	s.constructReference(c, 2)
	if e = m.squareFreeInto(s.refs[0].root.value, &s.refs[2].root); e != nil {
		return e
	}
	s.constructReference(c, 3)
	derivative, e := m.derivativeWithReceipt(s.refs[1].root.value, x, &s.derivativeInvocation)
	if e != nil {
		return e
	}
	if e = s.refs[3].root.assign(c, derivative); e != nil {
		return e
	}
	s.constructReference(c, 4)
	if e = m.gcd(s.refs[1].root.value, s.refs[3].root.value, &s.refs[4].root); e != nil {
		return e
	}
	constant, e = c.IsConstant(s.refs[4].root.value)
	if e != nil {
		return e
	}
	if constant {
		same, e := c.Equal(s.refs[2].root.value, s.refs[0].root.value)
		if e != nil {
			return e
		}
		if same {
			return out.assign(c, p)
		}
	} else {
		quotient, e := c.exactDivWithReceipt(s.refs[1].root.value, s.refs[4].root.value, &s.divisionInvocation)
		if e != nil {
			return e
		}
		if e = s.refs[1].root.assign(c, quotient); e != nil {
			return e
		}
	}
	unit, e := c.monomials.Unit()
	if e != nil {
		return e
	}
	scalar, e := c.scaleWithReceipt(&s.nums[0], unit, s.refs[2].root.value, &s.directArithmetic)
	if e != nil {
		return e
	}
	if e = out.assign(c, scalar); e != nil {
		return e
	}
	result, e := c.mulWithReceipt(out.value, s.refs[1].root.value, &s.directArithmetic)
	if e != nil {
		return e
	}
	return out.assign(c, result)
}
func (m *MultivariatePolynomialManager) squareFreeVariableInto(p Polynomial, x uint32, out *polynomialReference) (err error) {
	c := m.core
	zero, e := c.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		return out.assign(c, c.zero)
	}
	constant, e := c.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		return out.assign(c, p)
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	s.constructReference(c, 0)
	derivative, e := m.derivativeWithReceipt(p, x, &s.derivativeInvocation)
	if e != nil {
		return e
	}
	if e = s.refs[0].root.assign(c, derivative); e != nil {
		return e
	}
	s.constructReference(c, 1)
	if e = m.gcd(p, s.refs[0].root.value, &s.refs[1].root); e != nil {
		return e
	}
	constant, e = c.IsConstant(s.refs[1].root.value)
	if e != nil {
		return e
	}
	if constant {
		return out.assign(c, p)
	}
	result, e := c.exactDivWithReceipt(p, s.refs[1].root.value, &s.divisionInvocation)
	if e != nil {
		return e
	}
	return out.assign(c, result)
}
func (m *MultivariatePolynomialManager) IsSquareFree(p Polynomial) (bool, error) {
	return m.isSquareFree(p, 0, false)
}
func (m *MultivariatePolynomialManager) IsSquareFreeVariable(p Polynomial, x uint32) (bool, error) {
	return m.isSquareFree(p, x, true)
}
func (m *MultivariatePolynomialManager) isSquareFree(p Polynomial, x uint32, byVariable bool) (bool, error) {
	return m.isSquareFreeWithReceipt(p, x, byVariable, nil)
}
func (m *MultivariatePolynomialManager) isSquareFreeWithReceipt(p Polynomial, x uint32, byVariable bool, receipt *polynomialGCDInvocationReceipt) (result bool, err error) {
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
	s.constructReference(c, 0)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	if byVariable {
		e = m.squareFreeVariableInto(p, x, &s.refs[0].root)
	} else {
		e = m.squareFreeInto(p, &s.refs[0].root)
	}
	if e != nil {
		return false, e
	}
	line := int32(4567)
	if byVariable {
		line = 4596
	}
	if e = polynomialSquareFreeSourceAssert(line, func() (bool, error) {
		equal, e := c.Equal(p, s.refs[0].root.value)
		if e != nil {
			return false, e
		}
		return !equal || p == s.refs[0].root.value, nil
	}); e != nil {
		return false, e
	}
	return p == s.refs[0].root.value, nil
}
