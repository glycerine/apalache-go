package polynomial

// Source: algebraic_numbers.cpp:235-240,427,445-446,509-539.

func (m *AlgebraicRootCore) copySourceBasicValue(source *AlgebraicRootCore, l *algebraicLocation) (algebraicBasicValueView, error) {
	if _, _, e := algebraicAssertionSwitch(); e != nil {
		return algebraicBasicValueView{}, e
	}
	if l != nil {
		if e := algebraicQueryTagAssertionSwitch(); e != nil {
			return algebraicBasicValueView{}, e
		}
	}
	return algebraicBasicValueView{source: source, cell: l, zero: &m.zero}, nil
}
func (m *AlgebraicRootCore) assertCellCopyPolynomialEmpty(r *algebraicRecord) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && r.poly != (UPolynomial{}) {
		if e = m.cellAssertionFailure(445, "c->m_p == 0"); e != nil {
			return e
		}
	}
	_, enabled, e = algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && r.size != 0 {
		return m.cellAssertionFailure(446, "c->m_p_sz == 0")
	}
	return nil
}
func (m *AlgebraicRootCore) assertCellResetZero(a *Algebraic) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	if a.slot != 0 {
		return m.cellAssertionFailure(427, "is_zero(a)")
	}
	return nil
}
