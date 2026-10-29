package polynomial

// Source: src/math/polynomial/polynomial.cpp:4352-4506;
// src/math/polynomial/polynomial.h:859-868.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) GCD(u, v Polynomial, r *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	if r == nil || r.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	return m.gcd(u, v, &r.root)
}
func (m *MultivariatePolynomialManager) gcd(u, v Polynomial, r *polynomialReference) error {
	return m.gcdIntoWithReceipt(u, v, r, nil)
}
func (m *MultivariatePolynomialManager) gcdIntoWithReceipt(u, v Polynomial, r *polynomialReference, receipt *polynomialGCDInvocationReceipt) (err error) {
	c := m.core
	s, e := m.beginGCDFunction()
	if e != nil {
		return e
	}
	if receipt != nil {
		*receipt = polynomialGCDInvocationReceipt{m, s, s.invocation}
	}
	s.constructPowers(0)
	s.constructPowers(1)
	defer func() {
		s.bodyCause = err
		m.gcdDepth--
		err = errors.Join(err, m.finishGCDFunctionSource(s))
		c.pscLeave()
	}()
	if e := m.traceGCDEntry(u, v, s); e != nil {
		return e
	}
	zero, e := c.IsZero(u)
	if e != nil {
		return e
	}
	if zero {
		if e = r.assign(c, v); e != nil {
			return e
		}
		return m.flipLeadingSignWithReceipt(r, &s.directNeg)
	}
	zero, e = c.IsZero(v)
	if e != nil {
		return e
	}
	if zero {
		if e = r.assign(c, u); e != nil {
			return e
		}
		return m.flipLeadingSignWithReceipt(r, &s.directNeg)
	}
	if u == v {
		if e = r.assign(c, u); e != nil {
			return e
		}
		return m.flipLeadingSignWithReceipt(r, &s.directNeg)
	}
	constant, e := c.IsConstant(u)
	if e != nil {
		return e
	}
	if !constant {
		constant, e = c.IsConstant(v)
		if e != nil {
			return e
		}
	}
	if constant {
		s.constructNumeral(0)
		s.constructNumeral(1)
		if e = m.gcdScalarContent(s, v, &s.nums[1]); e != nil {
			return e
		}
		if e = m.gcdScalarContent(s, u, &s.nums[0]); e != nil {
			return e
		}
		s.constructNumeral(2)
		before, e := polynomialNumericBefore(c.integer)
		if e != nil {
			return e
		}
		e = c.modular.GCD(&s.nums[1], &s.nums[0], &s.nums[2])
		e = errors.Join(e, s.numeric.after(c.integer, before))
		if e != nil {
			return e
		}
		p, e := c.mkConstantTakeCopyWithArithmeticReceipt(&s.nums[2], &s.directConstant)
		if e != nil {
			return e
		}
		return r.assign(c, p)
	}
	if e = m.variableDegrees(u, &s.powers[0], true); e != nil {
		return e
	}
	s.powers[0].sortVariables()
	if e = m.variableDegrees(v, &s.powers[1], true); e != nil {
		return e
	}
	s.powers[1].sortVariables()
	if e := m.traceGCDVariables(s); e != nil {
		return e
	}
	x := ^uint32(0)
	uFound, vFound := false, false
	i := uint32(0)
	uSize, vSize := uint32(len(s.powers[0])), uint32(len(s.powers[1]))
	size := uSize
	if vSize < size {
		size = vSize
	}
	for ; i < size; i++ {
		xu, e := s.powers[0].word(i, 0)
		if e != nil {
			return e
		}
		xv, e := s.powers[1].word(i, 0)
		if e != nil {
			return e
		}
		if xu < xv {
			x = xu
			uFound = true
			break
		}
		if xu > xv {
			x = xv
			vFound = true
			break
		}
	}
	if !uFound && !vFound && i < uSize {
		x, e = s.powers[0].word(i, 0)
		if e != nil {
			return e
		}
		uFound = true
	}
	if !uFound && !vFound && i < vSize {
		x, e = s.powers[1].word(i, 0)
		if e != nil {
			return e
		}
		vFound = true
	}
	if uFound {
		return m.gcdContent(u, x, v, r)
	}
	if vFound {
		return m.gcdContent(v, x, u, r)
	}
	x, e = s.powers[0].word(size-1, 0)
	if e != nil {
		return e
	}
	if e = polynomialGCDDispatchSourceAssert(4447, "u_var_degrees[sz - 1].get_var() == v_var_degrees[sz - 1].get_var()", func() (bool, error) {
		other, e := s.powers[1].word(size-1, 0)
		return other == x, e
	}); e != nil {
		return e
	}
	if e = polynomialGCDDispatchSourceAssert(4448, "max_var(u) == max_var(v)", func() (bool, error) {
		a, e := c.MaxVariable(u)
		if e != nil {
			return false, e
		}
		b, e := c.MaxVariable(v)
		return a == b, e
	}); e != nil {
		return e
	}
	if e = polynomialGCDDispatchSourceAssert(4449, "max_var(u) == x", func() (bool, error) {
		a, e := c.MaxVariable(u)
		return a == x, e
	}); e != nil {
		return e
	}
	field, e := c.modular.Modular()
	if e != nil {
		return e
	}
	if !field && !m.usePRSGCD {
		if e = polynomialGCDDispatchSourceAssert(4468, "max_var(u) == max_var(v)", func() (bool, error) {
			a, e := c.MaxVariable(u)
			if e != nil {
				return false, e
			}
			b, e := c.MaxVariable(v)
			return a == b, e
		}); e != nil {
			return e
		}
		univariate, e := c.IsUnivariate(u)
		if e != nil {
			return e
		}
		if univariate {
			if e = polynomialGCDDispatchSourceAssert(4470, "is_univariate(v)", func() (bool, error) {
				ok, e := c.IsUnivariate(v)
				return ok, e
			}); e != nil {
				return e
			}
			return m.uniModGCD(u, v, r)
		}
		e = m.modGCDWithDebugCheck(u, v, x, r, s)
		// The default algorithm is the actual modular engine; only its source
		// sparse_mgcd_failed exception selects PRS fallback.
		if !m.gcdMayCatchSparse(s, e) {
			return e
		}
		old := m.usePRSGCD
		m.usePRSGCD = false
		defer func() { m.usePRSGCD = old }()
		return m.gcdPRS(u, v, x, r)
	}
	return m.gcdPRS(u, v, x, r)
}

// The source's exception is caught by type, not by a numeric error/category.
// Actual throw sites are in mod_gcd_rec; no generic arithmetic refusal uses it.
type polynomialSparseMGCDException struct{}

func (*polynomialSparseMGCDException) Error() string { return "polynomial sparse modular GCD failed" }

// The scalar IC overload contains only actual numeric leaves and coefficient
// reads; unlike polynomial-reference assignment, it invokes no external
// deletion callback. Its newly returned numeric roots belong to this call.
func (m *MultivariatePolynomialManager) gcdScalarContent(s *polynomialGCDFunctionScope, p Polynomial, out *unumeral.Int) error {
	before, e := polynomialNumericBefore(m.core.integer)
	if e != nil {
		return e
	}
	e = m.core.integerScalarContent(p, out)
	return errors.Join(e, s.numeric.after(m.core.integer, before))
}

// The source catches the sparse exception only after callee lexical unwind.
func (m *MultivariatePolynomialManager) gcdMayCatchSparse(s *polynomialGCDFunctionScope, e error) bool {
	var failed *polynomialSparseMGCDException
	if !errors.As(e, &failed) {
		return false
	}
	return m.gcdYoungerSourceStop(s) == nil
}
