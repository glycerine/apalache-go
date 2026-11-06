package polynomial

// Source: src/math/polynomial/polynomial.cpp:4235-4350.

import (
	"errors"
)

func (m *MultivariatePolynomialManager) modularGCDImage(s *polynomialGCDFunctionScope, prime uint32) (bad bool, err error) {
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
	size, e := c.readWord(s.refs[2].root.value, 8)
	if e != nil {
		return false, e
	}
	old, e := c.readWord(s.refs[0].root.value, 8)
	if e != nil {
		return false, e
	}
	if size != old {
		m.traceModGCDLiteral(4293, "bad prime, coefficient(s) vanished\n")
		return true, nil
	}
	v, e := m.normalizeWithReceipt(s.refs[1].root.value, &s.normalizeInvocation)
	if e != nil {
		return false, e
	}
	if e = s.refs[3].root.assign(c, v); e != nil {
		return false, e
	}
	size, e = c.readWord(s.refs[3].root.value, 8)
	if e != nil {
		return false, e
	}
	old, e = c.readWord(s.refs[1].root.value, 8)
	if e != nil {
		return false, e
	}
	if size != old {
		m.traceModGCDLiteral(4298, "bad prime, coefficient(s) vanished\n")
		return true, nil
	}
	if e = m.traceModGCDPolynomials("mgcd", 4301, "u_Zp: ", s.refs[2].root.value, s.refs[3].root.value); e != nil {
		return false, e
	}
	if e = m.modGCDRec(s.refs[2].root.value, s.refs[3].root.value, prime, 0, s.vars, &s.refs[5].root); e != nil {
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
func (m *MultivariatePolynomialManager) modGCD(u, v Polynomial, ud, vd *polynomialGCDPowers, r *polynomialReference) (err error) {
	c := m.core
	if e := m.newton.SetOwner(c); e != nil {
		return e
	}
	if err = m.traceModGCDPolynomials("mgcd", 4239, "mod_gcd\nu: ", u, v); err != nil {
		return err
	}
	if err = m.traceModGCDPolynomials("mgcd_call", 4240, "mod_gcd\nu: ", u, v); err != nil {
		return err
	}
	if err = polynomialModGCDSourceAssert(4241, "!m().modular()", func() (bool, error) {
		field, e := c.modular.Modular()
		return !field, e
	}); err != nil {
		return err
	}
	if err = polynomialModGCDSourceAssert(4243, "u_var_degrees.size() == v_var_degrees.size()", func() (bool, error) {
		return uint32(len(*ud)) == uint32(len(*vd)), nil
	}); err != nil {
		return err
	}
	size := uint32(len(*ud))
	if err = polynomialModGCDSourceAssert(4245, "num_vars > 1", func() (bool, error) {
		return size > 1, nil
	}); err != nil {
		return err
	}
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	s.vars = nil
	s.stack[s.depth] = gcdVariablesId
	s.depth++
	s.constructPowers(0)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	for i := uint32(0); i < size; i++ {
		if err = polynomialModGCDSourceAssert(4249, "u_var_degrees[i].get_var() == v_var_degrees[i].get_var()", func() (bool, error) {
			ux, e := ud.word(i, 0)
			if e != nil {
				return false, e
			}
			vx, e := vd.word(i, 0)
			return ux == vx, e
		}); err != nil {
			return err
		}
		x, e := ud.word(i, 0)
		if e != nil {
			return e
		}
		a, e := ud.word(i, 1)
		if e != nil {
			return e
		}
		b, e := vd.word(i, 1)
		if e != nil {
			return e
		}
		if b < a {
			a = b
		}
		if e = s.powers[0].push(x, a); e != nil {
			return e
		}
	}
	s.powers[0].sortDegrees()
	if e = m.skeletons.Reset(); e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		x, e := s.powers[0].word(i, 0)
		if e != nil {
			return e
		}
		s.vars = append(s.vars, x)
		if e = m.skeletons.Push(nil); e != nil {
			return e
		}
	}
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
	pu, e := c.gradedLexMaxPosition(s.refs[0].root.value)
	if e != nil {
		return e
	}
	pv, e := c.gradedLexMaxPosition(s.refs[1].root.value)
	if e != nil {
		return e
	}
	s.constructNumeral(3)
	s.constructNumeral(4)
	a, e := c.mutableCoefficientCore(s.refs[0].root.value, pu)
	if e != nil {
		return e
	}
	if e = m.gcdCopyNumeral(s, &a, &s.nums[3]); e != nil {
		return e
	}
	a, e = c.mutableCoefficientCore(s.refs[1].root.value, pv)
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
		before, e = polynomialNumericBefore(c.integer)
		if e != nil {
			return e
		}
		e = c.modular.SetUint64(&s.nums[7], uint64(prime))
		e = errors.Join(e, s.numeric.after(c.integer, before))
		if e != nil {
			return e
		}
		if e = m.traceModGCDPrime(&s.nums[7]); e != nil {
			return e
		}
		bad, e := m.modularGCDImage(s, prime)
		if e != nil {
			return e
		}
		if bad {
			continue
		}
		if e = m.traceModGCDPolynomials("mgcd", 4308, "new q:\n", s.refs[5].root.value); e != nil {
			return e
		}
		constant, e := c.IsConstant(s.refs[5].root.value)
		if e != nil {
			return e
		}
		if constant {
			m.traceModGCDLiteral(4310, "done, modular gcd is one\n")
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
			pc, e := c.gradedLexMaxPosition(s.refs[4].root.value)
			if e != nil {
				return e
			}
			mc, e := c.monomialCore(s.refs[4].root.value, pc)
			if e != nil {
				return e
			}
			pq, e := c.gradedLexMaxPosition(s.refs[5].root.value)
			if e != nil {
				return e
			}
			mq, e := c.monomialCore(s.refs[5].root.value, pq)
			if e != nil {
				return e
			}
			order, e := c.monomials.GradedLex(mq, mc)
			if e != nil {
				return e
			}
			if order < 0 {
				m.traceModGCDLiteral(4324, "discarding image\n")
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
				if e = m.traceModGCDPolynomials("mgcd", 4330, "new combined:\n", s.refs[4].root.value); e != nil {
					return e
				}
			}
		}
		candidate, e := m.normalizeWithReceipt(s.refs[4].root.value, &s.normalizeInvocation)
		if e != nil {
			return e
		}
		if e = s.refs[6].root.assign(c, candidate); e != nil {
			return e
		}
		if e = m.traceModGCDPolynomials("mgcd", 4334, "candidate:\n", s.refs[6].root.value); e != nil {
			return e
		}
		frontier := s.depth
		s.constructNumeral(9)
		pos, e := c.gradedLexMaxPosition(s.refs[6].root.value)
		if e != nil {
			return e
		}
		coeff, e := c.mutableCoefficientCore(s.refs[6].root.value, pos)
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
			m.traceModGCDLiteral(4340, "found GCD\n")
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
			return m.traceModGCDPolynomials("mgcd", 4343, "r: ", r.value)
		}
		if e = m.finishGCDLocalsTo(s, frontier); e != nil {
			return e
		}
	}
	x, e := c.MaxVariable(u)
	if e != nil {
		return e
	}
	return m.gcdPRS(u, v, x, r)
}
