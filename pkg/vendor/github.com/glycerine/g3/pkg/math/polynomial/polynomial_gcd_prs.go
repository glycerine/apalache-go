package polynomial

// Source: src/math/polynomial/polynomial.cpp:3541-3672.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) gcdPrimitivePart(p Polynomial, x uint32, out *polynomialReference) error {
	return m.gcdPrimitivePartWithReceipt(p, x, out, nil)
}
func (m *MultivariatePolynomialManager) gcdPrimitivePartWithReceipt(p Polynomial, x uint32, out *polynomialReference, receipt *polynomialGCDInvocationReceipt) (err error) {
	c := m.core
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	if receipt != nil {
		*receipt = polynomialGCDInvocationReceipt{m, s, s.invocation}
	}
	s.constructNumeral(0)
	s.constructReference(c, 0)
	s.constructReference(c, 1)
	e = m.iccpPartsWithReceipt(p, x, &s.nums[0], &s.refs[0].root, &s.refs[1].root, &s.iccpInvocation)
	if e == nil {
		// The actual source return moves result into its caller temporary. Its
		// c/i locals then die before the caller assignment can invoke old-DEC.
		s.returned = s.refs[1].root
		s.refs[1].root = polynomialReference{}
	}
	s.bodyCause = e
	m.gcdDepth--
	cleanup := m.finishGCDFunctionSource(s)
	c.pscLeave()
	if e != nil || cleanup != nil {
		return errors.Join(e, cleanup)
	}
	e = out.assign(c, s.returned.value)
	s.bodyCause = e
	cleanup = s.returned.closeValue(c, false)
	if cleanup != nil {
		s.cleanupCause = cleanup
		return errors.Join(e, cleanup)
	}
	return errors.Join(e, m.finishGCDFunctionSource(s))
}
func (m *MultivariatePolynomialManager) gcdContent(u Polynomial, x uint32, v Polynomial, r *polynomialReference) (err error) {
	c := m.core
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	s.constructNumeral(0)
	s.constructReference(c, 0)
	s.constructReference(c, 1)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	if e = m.iccpPartsWithReceipt(u, x, &s.nums[0], &s.refs[0].root, &s.refs[1].root, &s.iccpInvocation); e != nil {
		return e
	}
	unit, e := c.monomials.Unit()
	if e != nil {
		return e
	}
	q, e := c.scaleWithReceipt(&s.nums[0], unit, s.refs[0].root.value, &s.directArithmetic)
	if e != nil {
		return e
	}
	if e = s.refs[0].root.assign(c, q); e != nil {
		return e
	}
	return m.gcd(s.refs[0].root.value, v, r)
}
func (m *MultivariatePolynomialManager) gcdPRS(u, v Polynomial, x uint32, r *polynomialReference) (err error) {
	c := m.core
	if e := c.traceGCDPRSEntry(u, v, x); e != nil {
		return e
	}
	a, e := c.Degree(u, x)
	if e != nil {
		return e
	}
	b, e := c.Degree(v, x)
	if e != nil {
		return e
	}
	if a < b {
		u, v = v, u
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	// Source declaration order: i_u,i_v,c_u,c_v,pp_u,pp_v,d_a,
	// d_r,g,h,rem,new_h. The stack, not array position, defines destruction.
	s.constructNumeral(0)
	s.constructNumeral(1)
	for i := uint8(0); i < 4; i++ {
		s.constructReference(c, i)
	}
	s.constructNumeral(2)
	for i := uint8(4); i < 9; i++ {
		s.constructReference(c, i)
	}
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	cu, cv, pu, pv := &s.refs[0].root, &s.refs[1].root, &s.refs[2].root, &s.refs[3].root
	dr, g, h, rem, nh := &s.refs[4].root, &s.refs[5].root, &s.refs[6].root, &s.refs[7].root, &s.refs[8].root
	if e = m.iccpPartsWithReceipt(u, x, &s.nums[0], cu, pu, &s.iccpInvocation); e != nil {
		return e
	}
	if e = m.iccpPartsWithReceipt(v, x, &s.nums[1], cv, pv, &s.iccpInvocation); e != nil {
		return e
	}
	if e = m.gcd(cu.value, cv.value, dr); e != nil {
		return e
	}
	before, e := polynomialNumericBefore(c.integer)
	if e != nil {
		return e
	}
	e = c.modular.GCD(&s.nums[0], &s.nums[1], &s.nums[2])
	e = errors.Join(e, s.numeric.after(c.integer, before))
	if e != nil {
		return e
	}
	if e = c.traceGCDPRSContent(u, v, &s.nums[0], &s.nums[1], cu.value, cv.value, pu.value, pv.value, dr.value, &s.nums[2]); e != nil {
		return e
	}
	if e = g.assign(c, c.unit); e != nil {
		return e
	}
	if e = h.assign(c, c.unit); e != nil {
		return e
	}
	counter := uint32(0)
	for {
		if e = polynomialGCDPRSSourceAssert(func() (bool, error) {
			du, xerr := c.Degree(pu.value, x)
			if xerr != nil {
				return false, xerr
			}
			dv, xerr := c.Degree(pv.value, x)
			return du >= dv, xerr
		}); e != nil {
			return e
		}
		du, e := c.Degree(pu.value, x)
		if e != nil {
			return e
		}
		dv, e := c.Degree(pv.value, x)
		if e != nil {
			return e
		}
		delta := du - dv
		if e = c.traceGCDPRSIteration(counter, pu.value, pv.value, delta); e != nil {
			return e
		}
		counter++
		if e = c.exactPseudoRemainderWithReceipt(pu.value, pv.value, x, &s.refs[7], &s.resultantPseudo); e != nil {
			return e
		}
		zero, e := c.IsZero(rem.value)
		if e != nil {
			return e
		}
		if zero {
			if e = c.traceGCDPRSZero(pv.value); e != nil {
				return e
			}
			if e = m.flipLeadingSignWithReceipt(pv, &s.directNeg); e != nil {
				return e
			}
			if e = m.gcdPrimitivePart(pv.value, x, r); e != nil {
				return e
			}
			q, e := m.gcdScalarProduct(&s.nums[2], dr.value, r.value)
			if e != nil {
				return e
			}
			return r.assign(c, q)
		}
		constant, e := c.IsConstant(rem.value)
		if e != nil {
			return e
		}
		if constant {
			if e = c.traceGCDPRSConstant(rem.value, dr.value, &s.nums[2]); e != nil {
				return e
			}
			unit, e := c.monomials.Unit()
			if e != nil {
				return e
			}
			q, e := c.scaleWithReceipt(&s.nums[2], unit, dr.value, &s.directArithmetic)
			if e != nil {
				return e
			}
			return r.assign(c, q)
		}
		if e = pu.assign(c, pv.value); e != nil {
			return e
		}
		q, e := c.exactDivWithReceipt(rem.value, g.value, &s.divisionInvocation)
		if e != nil {
			return e
		}
		if e = pv.assign(c, q); e != nil {
			return e
		}
		for i := uint32(0); i < delta; i++ {
			q, e = c.exactDivWithReceipt(pv.value, h.value, &s.divisionInvocation)
			if e != nil {
				return e
			}
			if e = pv.assign(c, q); e != nil {
				return e
			}
		}
		q, e = m.gcdLeadingCoefficient(s, pu.value, x)
		if e != nil {
			return e
		}
		if e = g.assign(c, q); e != nil {
			return e
		}
		if e = nh.assign(c, c.unit); e != nil {
			return e
		}
		for i := uint32(0); i < delta; i++ {
			q, e = c.mulWithReceipt(nh.value, g.value, &s.directArithmetic)
			if e != nil {
				return e
			}
			if e = nh.assign(c, q); e != nil {
				return e
			}
		}
		if delta > 1 {
			for i := uint32(0); i < delta-1; i++ {
				q, e = c.exactDivWithReceipt(nh.value, h.value, &s.divisionInvocation)
				if e != nil {
					return e
				}
				if e = nh.assign(c, q); e != nil {
					return e
				}
			}
		}
		if e = h.assign(c, nh.value); e != nil {
			return e
		}
	}
}

// This is the actual scalar-times-two-polynomials overload reached by PRS;
// multiplying the polynomials first and then scaling would change source
// checkpoints, coefficient publication and lifetime.
func (m *MultivariatePolynomialManager) gcdScalarProduct(a *unumeral.Int, p, q Polynomial) (result Polynomial, err error) {
	c := m.core
	zero, e := c.modular.IsZero(a)
	if e != nil {
		return Polynomial{}, e
	}
	if !zero {
		zero, e = c.IsZero(p)
		if e != nil {
			return Polynomial{}, e
		}
	}
	if !zero {
		zero, e = c.IsZero(q)
		if e != nil {
			return Polynomial{}, e
		}
	}
	if zero {
		return c.zero, nil
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return Polynomial{}, e
	}
	s.constructNumeral(0)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	if e = c.som.reset(); e != nil {
		return Polynomial{}, e
	}
	if e = c.som.acquireArithmeticInvocation(&s.product); e != nil {
		return Polynomial{}, e
	}
	size, e := c.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	for i := uint32(0); i < size; i++ {
		if e = c.checkpoint(); e != nil {
			return Polynomial{}, e
		}
		ai, e := c.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		before, e := polynomialNumericBefore(c.integer)
		if e != nil {
			return Polynomial{}, e
		}
		e = c.modular.Mul(a, &ai, &s.nums[0])
		e = errors.Join(e, s.numeric.after(c.integer, before))
		if e != nil {
			return Polynomial{}, e
		}
		term, e := c.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = c.som.addMulWithArithmeticReceipt(&s.nums[0], term, q, &s.product); e != nil {
			return Polynomial{}, e
		}
	}
	return c.som.mkWithArithmeticReceipt(false, &s.product)
}
