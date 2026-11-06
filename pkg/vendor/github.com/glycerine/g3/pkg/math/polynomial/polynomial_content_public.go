package polynomial

// Source: src/math/polynomial/polynomial.cpp:7199-7210.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Content returns the complete polynomial content with respect to x,
// including its integer factor. PrimitivePart discards that content.
func (m *MultivariatePolynomialManager) Content(p Polynomial, x uint32, out *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	if out == nil || out.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	return m.contentWithReceipt(p, x, &out.root, nil)
}
func (m *MultivariatePolynomialManager) contentWithReceipt(p Polynomial, x uint32, out *polynomialReference, receipt *polynomialGCDInvocationReceipt) (err error) {
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	if receipt != nil {
		*receipt = polynomialGCDInvocationReceipt{m, s, s.invocation}
	}
	s.constructNumeral(0)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		m.core.pscLeave()
	}()
	if e = m.contentParts(p, x, &s.nums[0], out); e != nil {
		return e
	}
	one, e := m.core.modular.IsOne(&s.nums[0])
	if e != nil {
		return e
	}
	if one {
		return nil
	}
	unit, e := m.core.monomials.Unit()
	if e != nil {
		return e
	}
	value, e := m.core.scaleWithReceipt(&s.nums[0], unit, out.value, &s.directArithmetic)
	if e != nil {
		return e
	}
	return out.assign(m.core, value)
}

// The inner overload's pp reference ends before the outer wrapper tests i.
func (m *MultivariatePolynomialManager) contentParts(p Polynomial, x uint32, i *unumeral.Int, out *polynomialReference) (err error) {
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	s.constructReference(m.core, 0)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		m.core.pscLeave()
	}()
	return m.iccpPartsWithReceipt(p, x, i, out, &s.refs[0].root, &s.iccpInvocation)
}
