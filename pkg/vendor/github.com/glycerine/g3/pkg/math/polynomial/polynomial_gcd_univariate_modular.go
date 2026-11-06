package polynomial

// Source: src/math/polynomial/polynomial.cpp:3541-3553,3769-3865.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) gcdCopyNumeral(s *polynomialGCDFunctionScope, a, out *unumeral.Int) error {
	before, e := polynomialNumericBefore(m.core.integer)
	if e != nil {
		return e
	}
	e = m.core.modular.Copy(out, a)
	return errors.Join(e, s.numeric.after(m.core.integer, before))
}
func (m *MultivariatePolynomialManager) uniModImage(s *polynomialGCDFunctionScope, x, du, dv uint32) (bad bool, err error) {
	c := m.core
	frontier := s.depth
	if e := s.zp.construct(m, &s.nums[7]); e != nil {
		return false, e
	}
	s.stack[s.depth] = gcdZPId
	s.depth++
	defer func() { err = errors.Join(err, m.finishGCDLocalsTo(s, frontier)) }()
	u, e := m.normalizeWithReceipt(s.refs[0].root.value, &s.normalizeInvocation)
	if e != nil {
		return false, e
	}
	if e = s.refs[2].root.assign(c, u); e != nil {
		return false, e
	}
	v, e := m.normalizeWithReceipt(s.refs[1].root.value, &s.normalizeInvocation)
	if e != nil {
		return false, e
	}
	if e = s.refs[3].root.assign(c, v); e != nil {
		return false, e
	}
	d, e := c.Degree(s.refs[2].root.value, x)
	if e != nil {
		return false, e
	}
	if d < du {
		m.traceUniModGCDLiteral(3811, "bad prime, leading coefficient vanished\n")
		return true, nil
	}
	d, e = c.Degree(s.refs[3].root.value, x)
	if e != nil {
		return false, e
	}
	if d < dv {
		m.traceUniModGCDLiteral(3815, "bad prime, leading coefficient vanished\n")
		return true, nil
	}
	if e = m.gcdEuclid(s.refs[2].root.value, s.refs[3].root.value, &s.refs[5].root); e != nil {
		return false, e
	}
	q, e := m.glexMonicWithReceipt(s.refs[5].root.value, &s.directMonic)
	if e != nil {
		return false, e
	}
	if e = s.refs[5].root.assign(c, q); e != nil {
		return false, e
	}
	s.constructNumeral(8)
	if e = m.gcdCopyNumeral(s, &s.nums[5], &s.nums[8]); e != nil {
		return false, e
	}
	unit, e := c.monomials.Unit()
	if e != nil {
		return false, e
	}
	q, e = c.scaleWithReceipt(&s.nums[8], unit, s.refs[5].root.value, &s.directArithmetic)
	if e != nil {
		return false, e
	}
	return false, s.refs[5].root.assign(c, q)
}
func (m *MultivariatePolynomialManager) uniModGCD(u, v Polynomial, r *polynomialReference) (err error) {
	c := m.core
	if err = m.traceUniModGCDPolynomials(3770, "univ_modular_gcd\nu: ", u, v); err != nil {
		return err
	}
	if e := polynomialUniModGCDSourceAssert(3771, "!m().modular()", func() (bool, error) {
		field, x := c.modular.Modular()
		return !field, x
	}); e != nil {
		return e
	}
	if e := polynomialUniModGCDSourceAssert(3772, "is_univariate(u)", func() (bool, error) { return c.IsUnivariate(u) }); e != nil {
		return e
	}
	if e := polynomialUniModGCDSourceAssert(3773, "!is_const(u) && !is_const(v)", func() (bool, error) {
		constant, x := c.IsConstant(u)
		if x != nil || constant {
			return false, x
		}
		constant, x = c.IsConstant(v)
		return !constant, x
	}); e != nil {
		return e
	}
	if e := polynomialUniModGCDSourceAssert(3774, "max_var(u) == max_var(v)", func() (bool, error) {
		ux, xerr := c.MaxVariable(u)
		if xerr != nil {
			return false, xerr
		}
		vx, xerr := c.MaxVariable(v)
		return ux == vx, xerr
	}); e != nil {
		return e
	}
	x, e := c.MaxVariable(u)
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
	s.constructNumeral(1)
	s.constructReference(c, 0)
	s.constructReference(c, 1)
	if e = c.integerContentWithReceipt(u, &s.nums[0], &s.refs[0].root, &s.contentInvocation); e != nil {
		return e
	}
	if e = c.integerContentWithReceipt(v, &s.nums[1], &s.refs[1].root, &s.contentInvocation); e != nil {
		return e
	}
	s.constructNumeral(2)
	before, e := polynomialNumericBefore(c.integer)
	if e != nil {
		return e
	}
	e = c.modular.GCD(&s.nums[0], &s.nums[1], &s.nums[2])
	e = errors.Join(e, s.numeric.after(c.integer, before))
	if e != nil {
		return e
	}
	s.constructNumeral(3)
	s.constructNumeral(4)
	du, e := c.Degree(s.refs[0].root.value, x)
	if e != nil {
		return e
	}
	dv, e := c.Degree(s.refs[1].root.value, x)
	if e != nil {
		return e
	}
	a, e := c.UnivariateCoefficient(s.refs[0].root.value, du)
	if e != nil {
		return e
	}
	if e = m.gcdCopyNumeral(s, &a, &s.nums[3]); e != nil {
		return e
	}
	a, e = c.UnivariateCoefficient(s.refs[1].root.value, dv)
	if e != nil {
		return e
	}
	if e = m.gcdCopyNumeral(s, &a, &s.nums[4]); e != nil {
		return e
	}
	s.constructNumeral(5)
	before, e = polynomialNumericBefore(c.integer)
	if e != nil {
		return e
	}
	e = c.modular.GCD(&s.nums[3], &s.nums[4], &s.nums[5])
	e = errors.Join(e, s.numeric.after(c.integer, before))
	if e != nil {
		return e
	}
	s.constructReference(c, 2)
	s.constructReference(c, 3)
	s.constructReference(c, 4)
	s.constructNumeral(6)
	s.constructReference(c, 5)
	s.constructReference(c, 6)
	s.constructNumeral(7)
	for _, prime := range upolyBigPrimes {
		// The source uint32 prime is passed through its unsigned numeral setter.
		before, e = polynomialNumericBefore(c.integer)
		if e != nil {
			return e
		}
		e = c.modular.SetUint64(&s.nums[7], uint64(prime))
		e = errors.Join(e, s.numeric.after(c.integer, before))
		if e != nil {
			return e
		}
		if e = m.traceUniModGCDPrime(&s.nums[7]); e != nil {
			return e
		}
		bad, e := m.uniModImage(s, x, du, dv)
		if e != nil {
			return e
		}
		if bad {
			continue
		}
		if e = m.traceUniModGCDPolynomials(3825, "new q:\n", s.refs[5].root.value); e != nil {
			return e
		}
		constant, e := c.IsConstant(s.refs[5].root.value)
		if e != nil {
			return e
		}
		if constant {
			m.traceUniModGCDLiteral(3827, "done, modular gcd is one\n")
			one, e := c.modular.IsOne(&s.nums[2])
			if e != nil {
				return e
			}
			if one {
				return r.assign(c, s.refs[5].root.value)
			}
			result, e := c.mkConstantTakeCopyWithArithmeticReceipt(&s.nums[2], &s.directConstant)
			if e != nil {
				return e
			}
			return r.assign(c, result)
		}
		if s.refs[4].root.value == (Polynomial{}) {
			if e = s.refs[4].root.assign(c, s.refs[5].root.value); e != nil {
				return e
			}
			if e = m.gcdCopyNumeral(s, &s.nums[7], &s.nums[6]); e != nil {
				return e
			}
		} else {
			dq, e := c.Degree(s.refs[5].root.value, x)
			if e != nil {
				return e
			}
			ds, e := c.Degree(s.refs[4].root.value, x)
			if e != nil {
				return e
			}
			if dq < ds {
				m.traceUniModGCDLiteral(3841, "discarding image\n")
				if e = s.refs[4].root.assign(c, s.refs[5].root.value); e != nil {
					return e
				}
				if e = m.gcdCopyNumeral(s, &s.nums[7], &s.nums[6]); e != nil {
					return e
				}
			} else {
				if e = m.gcdCRAWithLexReceipt(s.refs[5].root.value, &s.nums[7], s.refs[4].root.value, &s.nums[6], &s.refs[4], &s.lexSort); e != nil {
					return e
				}
				if e = m.traceUniModGCDPolynomials(3847, "new combined:\n", s.refs[4].root.value); e != nil {
					return e
				}
			}
		}
		if e = m.gcdPrimitivePart(s.refs[4].root.value, x, &s.refs[6].root); e != nil {
			return e
		}
		if e = m.traceUniModGCDPolynomials(3851, "candidate:\n", s.refs[6].root.value); e != nil {
			return e
		}
		frontier := s.depth
		s.constructNumeral(9)
		d, e := c.Degree(s.refs[6].root.value, x)
		if e != nil {
			return e
		}
		coeff, e := c.UnivariateCoefficient(s.refs[6].root.value, d)
		if e != nil {
			return e
		}
		if e = m.gcdCopyNumeral(s, &coeff, &s.nums[9]); e != nil {
			return e
		}
		divides, e := c.modular.Divides(&s.nums[9], &s.nums[5])
		if e != nil {
			return e
		}
		if divides {
			divides, e = c.dividesWithReceipt(s.refs[6].root.value, s.refs[0].root.value, &s.dividesInvocation)
			if e != nil {
				return e
			}
		}
		if divides {
			divides, e = c.dividesWithReceipt(s.refs[6].root.value, s.refs[1].root.value, &s.dividesInvocation)
			if e != nil {
				return e
			}
		}
		if divides {
			m.traceUniModGCDLiteral(3857, "found GCD\n")
			unit, e := c.monomials.Unit()
			if e != nil {
				return e
			}
			result, e := c.scaleWithReceipt(&s.nums[2], unit, s.refs[6].root.value, &s.directArithmetic)
			if e != nil {
				return e
			}
			if e = r.assign(c, result); e != nil {
				return e
			}
			if e = m.flipLeadingSignWithReceipt(r, &s.directNeg); e != nil {
				return e
			}
			return m.traceUniModGCDPolynomials(3860, "r: ", r.value)
		}
		if e = m.finishGCDLocalsTo(s, frontier); e != nil {
			return e
		}
	}
	return m.gcdPRS(u, v, x, r)
}
