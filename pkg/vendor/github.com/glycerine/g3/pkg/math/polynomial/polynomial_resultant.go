package polynomial

// Source: src/math/polynomial/polynomial.cpp:5356-5480.

import (
	"errors"
)

func (m *MultivariatePolynomialManager) Resultant(p, q Polynomial, x uint32, result *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	if result == nil || result.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := result.open(); e != nil {
		return e
	}
	return m.resultant(p, q, x, &result.root)
}
func (m *MultivariatePolynomialManager) resultant(p, q Polynomial, x uint32, result *polynomialReference) error {
	return m.resultantWithReceipt(p, q, x, result, nil)
}
func (m *MultivariatePolynomialManager) resultantWithReceipt(p, q Polynomial, x uint32, result *polynomialReference, receipt *polynomialGCDInvocationReceipt) (err error) {
	c := m.core
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	if receipt != nil {
		*receipt = polynomialGCDInvocationReceipt{manager: m, frame: s, invocation: s.invocation}
	}
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	s.constructReference(c, 0)
	s.constructReference(c, 1)
	A, B := &s.refs[0].root, &s.refs[1].root
	if e = A.assign(c, p); e != nil {
		return e
	}
	if e = B.assign(c, q); e != nil {
		return e
	}
	if e = m.traceResultant(s, x, false); e != nil {
		return e
	}
	zero, e := c.IsZero(A.value)
	if e != nil {
		return e
	}
	if !zero {
		zero, e = c.IsZero(B.value)
		if e != nil {
			return e
		}
	}
	if zero {
		return result.assign(c, c.zero)
	}
	constant, e := c.IsConstant(A.value)
	if e != nil {
		return e
	}
	if constant {
		b, e := c.IsConstant(B.value)
		if e != nil {
			return e
		}
		if b {
			return result.assign(c, c.unit)
		}
		d, e := c.Degree(B.value, x)
		if e != nil {
			return e
		}
		return c.powerIntoWithReceipt(A.value, d, result, &s.resultantPower)
	}
	constant, e = c.IsConstant(B.value)
	if e != nil {
		return e
	}
	if constant {
		d, e := c.Degree(A.value, x)
		if e != nil {
			return e
		}
		return c.powerIntoWithReceipt(B.value, d, result, &s.resultantPower)
	}
	s.constructNumeral(0)
	s.constructNumeral(1)
	for i := uint8(2); i < 6; i++ {
		s.constructReference(c, i)
	}
	cA, cB, ppA, ppB := &s.refs[2].root, &s.refs[3].root, &s.refs[4].root, &s.refs[5].root
	if e = m.iccpPartsWithReceipt(A.value, x, &s.nums[0], cA, ppA, &s.iccpInvocation); e != nil {
		return e
	}
	if e = m.iccpPartsWithReceipt(B.value, x, &s.nums[1], cB, ppB, &s.iccpInvocation); e != nil {
		return e
	}
	unit, e := c.monomials.Unit()
	if e != nil {
		return e
	}
	v, e := c.scaleWithReceipt(&s.nums[0], unit, cA.value, &s.resultantArithmetic)
	if e != nil {
		return e
	}
	if e = cA.assign(c, v); e != nil {
		return e
	}
	unit, e = c.monomials.Unit()
	if e != nil {
		return e
	}
	v, e = c.scaleWithReceipt(&s.nums[1], unit, cB.value, &s.resultantArithmetic)
	if e != nil {
		return e
	}
	if e = cB.assign(c, v); e != nil {
		return e
	}
	s.constructReference(c, 6)
	t := &s.refs[6].root
	d, e := c.Degree(B.value, x)
	if e != nil {
		return e
	}
	if e = c.powerIntoWithReceipt(cA.value, d, cA, &s.resultantPower); e != nil {
		return e
	}
	d, e = c.Degree(A.value, x)
	if e != nil {
		return e
	}
	if e = c.powerIntoWithReceipt(cB.value, d, cB, &s.resultantPower); e != nil {
		return e
	}
	v, e = c.mulWithReceipt(cA.value, cB.value, &s.resultantArithmetic)
	if e != nil {
		return e
	}
	if e = t.assign(c, v); e != nil {
		return e
	}
	if e = A.assign(c, ppA.value); e != nil {
		return e
	}
	if e = B.assign(c, ppB.value); e != nil {
		return e
	}
	if e = m.traceResultant(s, x, true); e != nil {
		return e
	}
	sign := int32(1)
	degA, e := c.Degree(A.value, x)
	if e != nil {
		return e
	}
	degB, e := c.Degree(B.value, x)
	if e != nil {
		return e
	}
	if degA < degB {
		A.value, B.value = B.value, A.value
		if degA%2 == 1 && degB%2 == 1 {
			sign = -1
		}
	}
	for i := uint8(7); i < 11; i++ {
		s.constructReference(c, i)
	}
	R, g, h, newH := &s.refs[7].root, &s.refs[8].root, &s.refs[9].root, &s.refs[10].root
	if e = g.assign(c, c.unit); e != nil {
		return e
	}
	if e = h.assign(c, c.unit); e != nil {
		return e
	}
	for {
		if e = m.traceResultantIteration(s); e != nil {
			return e
		}
		degA, e = c.Degree(A.value, x)
		if e != nil {
			return e
		}
		degB, e = c.Degree(B.value, x)
		if e != nil {
			return e
		}
		if e = polynomialResultantSourceAssert(5427, "degA >= degB", func() (bool, error) {
			return degA >= degB, nil
		}); e != nil {
			return e
		}
		delta := degA - degB
		if degA%2 == 1 && degB%2 == 1 {
			sign = -sign
		}
		if e = c.exactPseudoRemainderWithReceipt(A.value, B.value, x, &s.refs[7], &s.resultantPseudo); e != nil {
			return e
		}
		if e = A.assign(c, B.value); e != nil {
			return e
		}
		v, e = c.exactDivWithReceipt(R.value, g.value, &s.divisionInvocation)
		if e != nil {
			return e
		}
		if e = B.assign(c, v); e != nil {
			return e
		}
		for i := uint32(0); i < delta; i++ {
			v, e = c.exactDivWithReceipt(B.value, h.value, &s.divisionInvocation)
			if e != nil {
				return e
			}
			if e = B.assign(c, v); e != nil {
				return e
			}
		}
		v, e = m.resultantLeadingCoefficient(s, A.value, x)
		if e != nil {
			return e
		}
		if e = g.assignWithFactoryReceipt(c, v, &s.resultantFactory); e != nil {
			return e
		}
		if e = newH.assign(c, c.unit); e != nil {
			return e
		}
		if e = c.powerIntoWithReceipt(g.value, delta, newH, &s.resultantPower); e != nil {
			return e
		}
		if delta > 1 {
			for i := uint32(0); i < delta-1; i++ {
				v, e = c.exactDivWithReceipt(newH.value, h.value, &s.divisionInvocation)
				if e != nil {
					return e
				}
				if e = newH.assign(c, v); e != nil {
					return e
				}
			}
		}
		if e = h.assign(c, newH.value); e != nil {
			return e
		}
		d, e = c.Degree(B.value, x)
		if e != nil {
			return e
		}
		if d == 0 {
			degA, e = c.Degree(A.value, x)
			if e != nil {
				return e
			}
			v, e = m.resultantLeadingCoefficient(s, B.value, x)
			if e != nil {
				return e
			}
			if e = newH.assignWithFactoryReceipt(c, v, &s.resultantFactory); e != nil {
				return e
			}
			if e = c.powerIntoWithReceipt(newH.value, degA, newH, &s.resultantPower); e != nil {
				return e
			}
			if degA > 1 {
				for i := uint32(0); i < degA-1; i++ {
					v, e = c.exactDivWithReceipt(newH.value, h.value, &s.divisionInvocation)
					if e != nil {
						return e
					}
					if e = newH.assign(c, v); e != nil {
						return e
					}
				}
			}
			if e = h.assign(c, newH.value); e != nil {
				return e
			}
			v, e = c.mulWithReceipt(t.value, h.value, &s.resultantArithmetic)
			if e != nil {
				return e
			}
			if e = result.assign(c, v); e != nil {
				return e
			}
			if sign < 0 {
				v, e = c.negWithReceipt(result.value, &s.resultantNeg)
				if e != nil {
					return e
				}
				return result.assign(c, v)
			}
			return nil
		}
	}
}
