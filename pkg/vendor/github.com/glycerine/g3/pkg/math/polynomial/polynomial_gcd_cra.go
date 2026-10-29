package polynomial

// Source: src/math/polynomial/polynomial.cpp:3675-3767.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The twelve actual scoped_numeral declarations, constructed at their source
// sites. The outer manager's reflected cold extent owns this concrete carrier.

type polynomialCRAScope struct {
	nums                    [12]unumeral.Int
	constructed             uint8
	active                  bool
	append                  polynomialCheapAppendInvocationReceipt
	factory                 polynomialFactoryInvocationReceipt
	numeric                 polynomialNumericRoots
	bodyCause, cleanupCause error
}

func (m *MultivariatePolynomialManager) craSourceEnd() error {
	s := &m.cra
	if s.cleanupCause != nil {
		return s.cleanupCause
	}
	for s.constructed != 0 {
		i := s.constructed - 1
		if _, e := unumeral.SourceNumeralDel(m.core.integer, &s.nums[i]); e != nil {
			s.cleanupCause = e
			return e
		}
		s.constructed--
	}
	pending := s.numeric.PendingObject().OwnerID() != 0 || m.core.cheap.matchesAppendInvocation(&s.append) || m.core.matchesFactoryInvocation(&s.factory)
	for _, raw := range s.numeric.PendingAllocatorBlocks() {
		pending = pending || raw.OwnerID() != 0
	}
	if !pending {
		*s = polynomialCRAScope{}
	}
	return nil
}
func (m *MultivariatePolynomialManager) cleanupCRACompleted() error {
	s := &m.cra
	for s.constructed != 0 {
		i := s.constructed - 1
		if e := m.core.deleteNumeral(&s.nums[i]); e != nil {
			return e
		}
		s.constructed--
	}
	return nil
}
func (m *MultivariatePolynomialManager) CleanupCRA() error {
	if m != nil && m.self == m && m.core != nil && m.core.quadraticTraceCallback() {
		return ErrPolynomialPending
	}
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if e := m.cleanupLexSortInvocation(&m.craLex); e != nil {
		return e
	}
	s := &m.cra
	if !s.active {
		return nil
	}
	c := m.core
	if e := m.cleanupCRACompleted(); e != nil {
		return e
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
	*s = polynomialCRAScope{}
	return nil
}

// Concrete operation dispatch shares the real numeric engines; the receipt
// brackets only the invoked arithmetic leaf, never caller callbacks.
func (m *MultivariatePolynomialManager) craLeaf(op uint8, a, b, out *unumeral.Int) error {
	c := m.core
	s := &m.cra
	before, e := polynomialNumericBefore(c.integer)
	if e != nil {
		return e
	}
	switch op {
	case 0:
		e = c.modular.Mul(a, b, out)
	case 1:
		e = c.modular.Add(a, b, out)
	case 2:
		e = c.modular.Sub(a, b, out)
	case 3:
		e = c.integer.Mod(a, b, out)
	case 4:
		e = c.modular.Div(a, b, out)
	case 5:
		e = c.modular.Copy(out, a)
	case 6:
		e = c.modular.Neg(a)
	}
	return errors.Join(e, s.numeric.after(c.integer, before))
}
func (m *MultivariatePolynomialManager) craAdd(a Polynomial, ai uint32, b Polynomial, bi uint32, term Monomial, monoFrom Polynomial, mi uint32) error {
	c := m.core
	s := &m.cra
	n := &s.nums
	av := unumeral.NewInt(0)
	if a != (Polynomial{}) {
		var e error
		av, e = c.coefficientCore(a, ai)
		if e != nil {
			return e
		}
	}
	if e := m.craLeaf(0, &av, &n[3], &n[9]); e != nil {
		return e
	}
	bv := unumeral.NewInt(0)
	if b != (Polynomial{}) {
		var e error
		bv, e = c.coefficientCore(b, bi)
		if e != nil {
			return e
		}
	}
	if e := m.craLeaf(0, &bv, &n[4], &n[10]); e != nil {
		return e
	}
	if e := m.craLeaf(1, &n[9], &n[10], &n[11]); e != nil {
		return e
	}
	if e := m.craLeaf(3, &n[11], &n[5], &n[8]); e != nil {
		return e
	}
	gt, e := c.modular.Greater(&n[8], &n[7])
	if e != nil {
		return e
	}
	if gt {
		if e = m.craLeaf(2, &n[8], &n[5], &n[8]); e != nil {
			return e
		}
	}
	if monoFrom != (Polynomial{}) {
		term, e = c.monomialCore(monoFrom, mi)
		if e != nil {
			return e
		}
	}
	return c.cheap.addWithAppendReceipt(&n[8], term, false, &m.cra.append)
}
func (m *MultivariatePolynomialManager) gcdCRA(a Polynomial, b1 *unumeral.Int, b Polynomial, b2 *unumeral.Int, out *PolynomialRef) error {
	return m.gcdCRAWithLexReceipt(a, b1, b, b2, out, &m.craLex)
}
func (m *MultivariatePolynomialManager) gcdCRAWithLexReceipt(a Polynomial, b1 *unumeral.Int, b Polynomial, b2 *unumeral.Int, out *PolynomialRef, lex *polynomialLexSortInvocationReceipt) (err error) {
	if e := m.ready(); e != nil {
		return e
	}
	if out == nil || out.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	c := m.core
	if e := m.lexSortWithReceipt(a, nil, lex); e != nil {
		return e
	}
	if e := m.lexSortWithReceipt(b, nil, lex); e != nil {
		return e
	}
	if e := m.traceCRAImages(a, b); e != nil {
		return e
	}
	if e := polynomialCRASourceAssert(3679, "m_cheap_som_buffer.empty()", func() (bool, error) {
		size, x := c.cheap.ms.size(c)
		return size == 0, x
	}); e != nil {
		return e
	}
	if e := polynomialCRASourceAssert(3680, "!m().m().is_even(b1)", func() (bool, error) {
		even, x := c.integer.IsEven(b1)
		return !even, x
	}); e != nil {
		return e
	}
	if e := polynomialCRASourceAssert(3681, "!m().m().is_even(b2)", func() (bool, error) {
		even, x := c.integer.IsEven(b2)
		return !even, x
	}); e != nil {
		return e
	}
	s := &m.cra
	if s.active {
		return ErrPolynomialPending
	}
	if e := c.pscEnter(); e != nil {
		return e
	}
	*s = polynomialCRAScope{active: true, constructed: 3}
	defer func() { s.bodyCause = err; err = errors.Join(err, m.craSourceEnd()); c.pscLeave() }()
	n := &s.nums
	before, e := polynomialNumericBefore(c.integer)
	if e != nil {
		return e
	}
	e = c.modular.ExtendedGCD(b1, b2, &n[0], &n[1], &n[2])
	e = errors.Join(e, s.numeric.after(c.integer, before))
	if e != nil {
		return e
	}
	if e = polynomialCRASourceAssert(3687, "m().is_one(g)", func() (bool, error) { return c.modular.IsOne(&n[2]) }); e != nil {
		return e
	}
	if e = m.traceCRABezout(b1, b2, &n[0], &n[1]); e != nil {
		return e
	}
	if e = m.craLeaf(3, &n[0], b2, &n[0]); e != nil {
		return e
	}
	if e = m.craLeaf(3, &n[1], b1, &n[1]); e != nil {
		return e
	}
	if e = m.traceCRAInverse(&n[0], &n[1]); e != nil {
		return e
	}
	s.constructed = 5
	if e = m.craLeaf(0, b2, &n[1], &n[3]); e != nil {
		return e
	}
	if e = m.craLeaf(0, b1, &n[0], &n[4]); e != nil {
		return e
	}
	if e = m.traceCRAMultipliers(&n[3], &n[4]); e != nil {
		return e
	}
	s.constructed = 6
	if e = m.craLeaf(0, b1, b2, &n[5]); e != nil {
		return e
	}
	s.constructed = 12
	two := unumeral.NewInt(2)
	if e = m.craLeaf(4, &n[5], &two, &n[7]); e != nil {
		return e
	}
	if e = m.craLeaf(5, &n[7], nil, &n[6]); e != nil {
		return e
	}
	if e = m.craLeaf(6, &n[6], nil, nil); e != nil {
		return e
	}
	if e = m.traceCRABounds(&n[6], &n[7]); e != nil {
		return e
	}
	sa, e := c.readWord(a, 8)
	if e != nil {
		return e
	}
	sb, e := c.readWord(b, 8)
	if e != nil {
		return e
	}
	var ia, ib uint32
	for {
		if ia == sa {
			for ib < sb {
				m.traceCRABranch(3729, "adding C2 rest\n")
				if e = m.craAdd(Polynomial{}, 0, b, ib, Monomial{}, b, ib); e != nil {
					return e
				}
				ib++
			}
			break
		}
		if ib == sb {
			for ia < sa {
				m.traceCRABranch(3737, "adding C1 rest\n")
				if e = m.craAdd(a, ia, Polynomial{}, 0, Monomial{}, a, ia); e != nil {
					return e
				}
				ia++
			}
			break
		}
		ma, e := c.monomialCore(a, ia)
		if e != nil {
			return e
		}
		mb, e := c.monomialCore(b, ib)
		if e != nil {
			return e
		}
		order, e := c.monomials.Lex(ma, mb)
		if e != nil {
			return e
		}
		if order == 0 {
			e = m.craAdd(a, ia, b, ib, ma, Polynomial{}, 0)
			if e == nil {
				e = m.traceCRAEqual(a, ia, b, ib, &n[8], &n[9], &n[10], &n[11])
			}
			ia++
			ib++
		} else if order > 0 {
			m.traceCRABranch(3755, "C1 mon biggerr, adding it...\n")
			e = m.craAdd(a, ia, Polynomial{}, 0, ma, Polynomial{}, 0)
			ia++
		} else {
			m.traceCRABranch(3760, "C2 mon bigger, adding it...\n")
			e = m.craAdd(Polynomial{}, 0, b, ib, mb, Polynomial{}, 0)
			ib++
		}
		if e != nil {
			return e
		}
	}
	if e = m.craLeaf(5, &n[5], nil, b2); e != nil {
		return e
	}
	result, e := c.cheap.mkWithAppendReceipt(false, &s.append, &s.factory)
	if e != nil {
		return e
	}
	return out.root.assignWithFactoryReceipt(c, result, &s.factory)
}
