package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// These scalar reads follow polynomial.cpp's comparator field order. They do
// not inspect unrelated refcount, ID or hash fields.
func (m *MonomialManager) orderField(a Monomial, offset uint64) (uint32, error) {
	l, e := m.location(a)
	if e != nil {
		return 0, e
	}
	return m.allocator.ReadUint32(l.raw, offset)
}

// Source get_power creates a reference and checks its index in debug. It
// does not eagerly load either power field.
func (m *MonomialManager) orderPowerLocation(a Monomial, i uint32) (*monomialLocation, error) {
	l, e := m.location(a)
	if e != nil {
		return nil, e
	}
	if e = m.powerSourceIndexAssert(a, i); e != nil {
		return nil, e
	}
	return l, nil
}
func (m *MonomialManager) orderPowerVar(l *monomialLocation, i uint32) (uint32, error) {
	return m.allocator.ReadUint32(l.raw, 20+uint64(i)*8)
}
func (m *MonomialManager) orderPowerDegree(l *monomialLocation, i uint32) (uint32, error) {
	return m.allocator.ReadUint32(l.raw, 24+uint64(i)*8)
}
func orderSignedIndex(n uint32) (int32, error) {
	s := int32(n)
	if s == -2147483648 {
		return 0, arena.ErrBounds
	}
	return s - 1, nil
} // Actual source signed-subtraction UB boundary.
func (m *MonomialManager) lexCore(a, b Monomial, minimum bool, minVar uint32) (int, error) {
	if a == b {
		return 0, nil
	}
	n1, e := m.orderField(a, 12)
	if e != nil {
		return 0, e
	}
	n2, e := m.orderField(b, 12)
	if e != nil {
		return 0, e
	}
	i, e := orderSignedIndex(n1)
	if e != nil {
		return 0, e
	}
	j, e := orderSignedIndex(n2)
	if e != nil {
		return 0, e
	}
	var d1, d2 uint32
	for i >= 0 && j >= 0 {
		p, e := m.orderPowerLocation(a, uint32(i))
		if e != nil {
			return 0, e
		}
		q, e := m.orderPowerLocation(b, uint32(j))
		if e != nil {
			return 0, e
		}
		v1, e := m.orderPowerVar(p, uint32(i))
		if e != nil {
			return 0, e
		}
		if minimum && v1 == minVar {
			d1, e = m.orderPowerDegree(p, uint32(i))
			if e != nil {
				return 0, e
			}
			i--
			v2, e := m.orderPowerVar(q, uint32(j))
			if e != nil {
				return 0, e
			}
			if v2 == minVar {
				d2, e = m.orderPowerDegree(q, uint32(j))
				if e != nil {
					return 0, e
				}
				j--
			}
			continue
		}
		v2, e := m.orderPowerVar(q, uint32(j))
		if e != nil {
			return 0, e
		}
		if minimum && v2 == minVar {
			d2, e = m.orderPowerDegree(q, uint32(j))
			if e != nil {
				return 0, e
			}
			j--
			continue
		}
		if v1 == v2 {
			x, e := m.orderPowerDegree(p, uint32(i))
			if e != nil {
				return 0, e
			}
			y, e := m.orderPowerDegree(q, uint32(j))
			if e != nil {
				return 0, e
			}
			if x == y {
				i--
				j--
				continue
			}
			if x < y {
				return -1, nil
			}
			return 1, nil
		}
		if v1 > v2 {
			return 1, nil
		}
		return -1, nil
	}

	if minimum {
		if i == j {
			if e = m.lexMinimumEndAssert(d1, d2); e != nil {
				return 0, e
			}
			if d1 < d2 {
				return -1, nil
			}
			return 1, nil
		}
	} else {
		if e = m.lexDistinctEndAssert(i, j); e != nil {
			return 0, e
		}
	}
	if i < 0 {
		return -1, nil
	}
	return 1, nil
}

// Lex preserves the source pointer-equality shortcut before any numeral reads.
func (m *MonomialManager) Lex(a, b Monomial) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.lexCore(a, b, false, 0)
}
func (m *MonomialManager) LexWithMinimum(a, b Monomial, minVar uint32) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.lexCore(a, b, true, minVar)
}
func (m *MonomialManager) GradedLex(a, b Monomial) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	d1, e := m.orderField(a, 8)
	if e != nil {
		return 0, e
	}
	d2, e := m.orderField(b, 8)
	if e != nil {
		return 0, e
	}
	if d1 == d2 {
		return m.lexCore(a, b, false, 0)
	}
	if d1 < d2 {
		return -1, nil
	}
	return 1, nil
}
func (m *MonomialManager) reverseRange(a Monomial, start1, end1 uint32, b Monomial, start2, end2 uint32) (int, error) {
	if polynomialSourceDebug && (end1 < start1 || end2 < start2) {
		return 0, ErrMonomialAssertion
	}
	i, j := end1, end2
	for i > start1 && j > start2 {
		i--
		j--
		p, e := m.orderPowerLocation(a, i)
		if e != nil {
			return 0, e
		}
		q, e := m.orderPowerLocation(b, j)
		if e != nil {
			return 0, e
		}
		v1, e := m.orderPowerVar(p, i)
		if e != nil {
			return 0, e
		}
		v2, e := m.orderPowerVar(q, j)
		if e != nil {
			return 0, e
		}
		if v1 == v2 {
			x, e := m.orderPowerDegree(p, i)
			if e != nil {
				return 0, e
			}
			y, e := m.orderPowerDegree(q, j)
			if e != nil {
				return 0, e
			}
			if x == y {
				if polynomialSourceDebug && !(i > start1 && j > start2) {
					return 0, ErrMonomialAssertion
				}
				continue
			}
			if x > y {
				return -1, nil
			}
			return 1, nil
		}
		if v1 > v2 {
			return -1, nil
		}
		return 1, nil
	}

	if polynomialSourceDebug && !(i == start1 || j == start2) {
		return 0, ErrMonomialAssertion
	}
	if i == start1 {
		if j == start2 {
			return 0, nil
		}
		return -1, nil
	}
	if polynomialSourceDebug && !(j == start2 && i != start1) {
		return 0, ErrMonomialAssertion
	}
	return 1, nil
}

// ReverseLexRange exposes the literal source submonomial algorithm. Its debug
// assertions include the source same-grade/distinct-submonomial assumptions.
func (m *MonomialManager) ReverseLexRange(a Monomial, start1, end1 uint32, b Monomial, start2, end2 uint32) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.reverseRange(a, start1, end1, b, start2, end2)
}
func (m *MonomialManager) reverseCore(a, b Monomial) (int, error) {
	if a == b {
		return 0, nil
	}
	n1, e := m.orderField(a, 12)
	if e != nil {
		return 0, e
	}
	n2, e := m.orderField(b, 12)
	if e != nil {
		return 0, e
	}
	return m.reverseRange(a, 0, n1, b, 0, n2)
}
func (m *MonomialManager) ReverseLex(a, b Monomial) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.reverseCore(a, b)
}
func (m *MonomialManager) GradedReverseLex(a, b Monomial) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	d1, e := m.orderField(a, 8)
	if e != nil {
		return 0, e
	}
	d2, e := m.orderField(b, 8)
	if e != nil {
		return 0, e
	}
	if d1 == d2 {
		return m.reverseCore(a, b)
	}
	if d1 < d2 {
		return -1, nil
	}
	return 1, nil
}
