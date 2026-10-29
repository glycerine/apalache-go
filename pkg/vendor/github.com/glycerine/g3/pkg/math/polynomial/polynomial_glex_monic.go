package polynomial

// Source: src/math/polynomial/polynomial.cpp:6843-6862,7388-7390;
// src/math/polynomial/polynomial.h:986.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialMonicScope struct {
	invocation              uint64
	executing               bool
	inv, newA               unumeral.Int
	numeric                 polynomialNumericRoots
	active                  bool
	cursor                  uint8
	append                  polynomialCheapAppendInvocationReceipt
	factory                 polynomialFactoryInvocationReceipt
	bodyCause, cleanupCause error
}

func (m *MultivariatePolynomialManager) finishGlexMonicSource() (err error) {
	s := &m.monic
	if !s.active {
		return nil
	}
	if s.cleanupCause != nil {
		return s.cleanupCause
	}
	defer func() {
		if err != nil {
			s.cleanupCause = err
		}
	}()
	if s.cursor == 0 {
		if _, e := unumeral.SourceNumeralDel(m.core.integer, &s.newA); e != nil {
			return e
		}
		s.cursor = 1
	}
	if s.cursor == 1 {
		if _, e := unumeral.SourceNumeralDel(m.core.integer, &s.inv); e != nil {
			return e
		}
		s.cursor = 2
	}
	return nil
}
func (m *MultivariatePolynomialManager) CleanupGlexMonic() error {
	if m == nil || m.self != m || m.core == nil {
		return ErrPolynomialOwner
	}
	return m.cleanupGlexMonicPhase(true)
}
func (m *MultivariatePolynomialManager) cleanupGlexMonicPhase(abort bool) error {
	s := &m.monic
	if !s.active {
		return nil
	}
	if s.executing {
		return ErrPolynomialPending
	}
	c := m.core
	if e := c.deleteNumeral(&s.newA); e != nil {
		return e
	}
	if e := c.deleteNumeral(&s.inv); e != nil {
		return e
	}
	if !abort {
		return nil
	}
	if e := s.numeric.cleanup(c.integer); e != nil {
		return e
	}
	if e := c.cleanupFactoryInvocation(&s.factory, true); e != nil {
		return e
	}
	if e := c.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
		return e
	}
	*s = polynomialMonicScope{invocation: s.invocation, bodyCause: s.bodyCause, cleanupCause: s.cleanupCause}
	return nil
}
func (m *MultivariatePolynomialManager) GlexMonic(p Polynomial) (result Polynomial, err error) {
	return m.glexMonicWithReceipt(p, nil)
}
func (m *MultivariatePolynomialManager) glexMonicWithReceipt(p Polynomial, receipt *polynomialMonicInvocationReceipt) (result Polynomial, err error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	c := m.core
	if e := polynomialGlexMonicSourceAssert(6844, "m_manager.field()", func() (bool, error) {
		field, e := c.modular.Field()
		return field, e
	}); e != nil {
		return Polynomial{}, e
	}
	zero, e := c.IsZero(p)
	if e != nil {
		return Polynomial{}, e
	}
	if zero {
		return p, nil
	}
	pos, e := c.gradedLexMaxPosition(p)
	if e != nil {
		return Polynomial{}, e
	}
	a, e := c.coefficientCore(p, pos)
	if e != nil {
		return Polynomial{}, e
	}
	one, e := c.modular.IsOne(&a)
	if e != nil {
		return Polynomial{}, e
	}
	if one {
		return p, nil
	}
	s := &m.monic
	if s.active || s.invocation == ^uint64(0) {
		return Polynomial{}, ErrPolynomialPending
	}
	if e = c.pscEnter(); e != nil {
		return Polynomial{}, e
	}
	*s = polynomialMonicScope{active: true, executing: true, invocation: s.invocation + 1}
	if receipt != nil {
		*receipt = polynomialMonicInvocationReceipt{m, s, s.invocation}
	}
	defer func() {
		s.executing = false
		c.pscLeave()
		s.bodyCause = err
		if e := m.finishGlexMonicSource(); e != nil {
			result = Polynomial{}
			err = errors.Join(err, e)
			return
		}
		if err == nil {
			if e := c.transferFactoryResult(&s.factory, result); e != nil {
				result = Polynomial{}
				err = e
				return
			}
		}
		if s.newA.NativeCellToken() == 0 && s.inv.NativeCellToken() == 0 && s.numeric.PendingObject().OwnerID() == 0 && !c.matchesFactoryInvocation(&s.factory) && !c.cheap.matchesAppendInvocation(&s.append) {
			hasRaw := false
			for _, p := range s.numeric.PendingAllocatorBlocks() {
				hasRaw = hasRaw || p.OwnerID() != 0
			}
			if !hasRaw {
				*s = polynomialMonicScope{invocation: s.invocation, bodyCause: s.bodyCause, cleanupCause: s.cleanupCause}
			}
		}
	}()
	a, e = c.coefficientCore(p, pos)
	if e != nil {
		return Polynomial{}, e
	}
	before, e := polynomialNumericBefore(c.integer)
	if e != nil {
		return Polynomial{}, e
	}
	e = c.modular.Copy(&s.inv, &a)
	e = polynomialNewtonJoin(e, s.numeric.after(c.integer, before))
	if e != nil {
		return Polynomial{}, e
	}
	before, e = polynomialNumericBefore(c.integer)
	if e != nil {
		return Polynomial{}, e
	}
	e = c.modular.Inv(&s.inv)
	e = polynomialNewtonJoin(e, s.numeric.after(c.integer, before))
	if e != nil {
		return Polynomial{}, e
	}
	if e = c.cheap.reset(); e != nil {
		return Polynomial{}, e
	}
	n, e := c.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	for i := uint32(0); i < n; i++ {
		a, e := c.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		before, e = polynomialNumericBefore(c.integer)
		if e != nil {
			return Polynomial{}, e
		}
		e = c.modular.Copy(&s.newA, &a)
		e = polynomialNewtonJoin(e, s.numeric.after(c.integer, before))
		if e != nil {
			return Polynomial{}, e
		}
		before, e = polynomialNumericBefore(c.integer)
		if e != nil {
			return Polynomial{}, e
		}
		e = c.modular.Mul(&s.newA, &s.inv, &s.newA)
		e = polynomialNewtonJoin(e, s.numeric.after(c.integer, before))
		if e != nil {
			return Polynomial{}, e
		}
		mono, e := c.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		before, e = polynomialNumericBefore(c.integer)
		if e != nil {
			return Polynomial{}, e
		}
		e = c.cheap.addWithAppendReceipt(&s.newA, mono, false, &s.append)
		e = polynomialNewtonJoin(e, s.numeric.after(c.integer, before))
		if e != nil {
			return Polynomial{}, e
		}
	}
	before, e = polynomialNumericBefore(c.integer)
	if e != nil {
		return Polynomial{}, e
	}
	result, e = c.cheap.mkWithAppendReceipt(false, &s.append, &s.factory)
	return result, polynomialNewtonJoin(e, s.numeric.after(c.integer, before))
}
