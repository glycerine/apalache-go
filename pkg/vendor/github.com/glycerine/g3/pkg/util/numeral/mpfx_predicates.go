package numeral

// Source: src/util/mpfx.h:160-247 and src/util/mpfx.cpp:316-417.

import "math/big"

func (m *FixedPointManager) IsPos(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return !a.sourceIsZero() && !a.sourceNegative(), nil
}

func (m *FixedPointManager) IsNeg(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.sourceNegative(), nil
}

func (m *FixedPointManager) IsNonPos(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.sourceIsZero() || a.sourceNegative(), nil
}

func (m *FixedPointManager) IsNonNeg(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return !a.sourceNegative(), nil
}

func (m *FixedPointManager) IsOne(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.scaled.Cmp(&m.scale) == 0, nil
}

func (m *FixedPointManager) IsMinusOne(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	var minusOne big.Int
	minusOne.Neg(&m.scale)
	return a.scaled.Cmp(&minusOne) == 0, nil
}

func (m *FixedPointManager) Equal(a, b *FixedPoint) (bool, error) {
	cmp, err := m.Compare(a, b)
	return cmp == 0, err
}

func (m *FixedPointManager) Less(a, b *FixedPoint) (bool, error) {
	if err := m.pair(a, b); err != nil {
		return false, err
	}
	m.traceFixedLess(a, b, false, false)
	less := compareFixedPointSource(a, b) < 0
	m.traceFixedLess(a, b, true, less)
	return less, nil
}

func (m *FixedPointManager) LessEqual(a, b *FixedPoint) (bool, error) {
	less, err := m.Less(b, a)
	if err != nil {
		return false, err
	}
	return !less, nil
}

func (m *FixedPointManager) Greater(a, b *FixedPoint) (bool, error) {
	return m.Less(b, a)
}

func (m *FixedPointManager) GreaterEqual(a, b *FixedPoint) (bool, error) {
	less, err := m.Less(a, b)
	if err != nil {
		return false, err
	}
	return !less, nil
}

// Inc and Dec are source add/sub with the manager's retained one numeral.
func (m *FixedPointManager) Inc(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	var next big.Int
	next.Add(&a.scaled, &m.scale)
	if err := m.publish(a, &next); err != nil {
		m.publishAddOverflow(a, &next)
		return err
	}
	return nil
}

func (m *FixedPointManager) Dec(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	var next big.Int
	next.Sub(&a.scaled, &m.scale)
	if err := m.publish(a, &next); err != nil {
		m.publishAddOverflow(a, &next)
		return err
	}
	return nil
}

// Swap exchanges owned values without exchanging their manager bindings.
func (m *FixedPointManager) Swap(a, b *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if a == b {
		return nil
	}
	var temp big.Int
	temp.Set(&a.scaled)
	tempSlot, tempNegative := a.sourceZeroSlot, a.sourceZeroNegative
	a.scaled.Set(&b.scaled)
	a.sourceZeroSlot, a.sourceZeroNegative = b.sourceZeroSlot, b.sourceZeroNegative
	b.scaled.Set(&temp)
	b.sourceZeroSlot, b.sourceZeroNegative = tempSlot, tempNegative
	return nil
}
