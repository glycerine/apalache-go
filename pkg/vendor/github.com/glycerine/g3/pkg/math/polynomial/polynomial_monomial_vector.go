package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import "sort"

func (m *MonomialManager) powerVectorCleanup() error       { m.powers = nil; return nil }
func (m *MonomialManager) powerVectorSourceDestroy() error { m.powers = nil; return nil }
func (m *MonomialManager) MkFromVariables(xs []uint32) (Monomial, error) {
	if e := m.check(); e != nil {
		return Monomial{}, e
	}
	if len(xs) == 0 {
		return m.unit, nil
	}
	if len(xs) == 1 {
		return m.mkVariableCreatedAt(xs[0], 1, nil, 885)
	}
	m.powers = m.powers[:0]
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	for i, x := range xs {
		line := int32(909)
		if i == 0 {
			line = 905
		}
		if e := m.sourceScalarAssert(line, "is_valid(x)", x < m.nextVar); e != nil {
			return Monomial{}, e
		}
		n := len(m.powers)
		if n != 0 && m.powers[n-1].Var == x {
			m.powers[n-1].Degree++
		} else {
			m.powers = append(m.powers, MonomialPower{x, 1})
		}
	}
	if e := m.tempInit(&m.temps[0], m.powers); e != nil {
		return Monomial{}, e
	}
	return m.internTemp(0)
}
