package numeral

// Source: src/util/mpzzp.h numeric manager members, with caller-owned cursors.

// FinalizeSourceMembers runs the literal source member destructions in order.
// The containing caller retains its destructor cursor across transport stops.
func (m *IntegerModularManager) FinalizeSourceMembers(cursor *uint8) error {
	if m == nil || !m.ValidOwner() || cursor == nil {
		return ErrModularOwner
	}
	fields := [7]*Int{&m.P, &m.Lower, &m.Upper, &m.InvTmp1, &m.InvTmp2, &m.InvTmp3, &m.DivTmp}
	for *cursor < uint8(len(fields)) {
		if _, e := SourceNumeralDel(m.manager, fields[*cursor]); e != nil {
			return e
		}
		*cursor++
	}
	return nil
}
func (m *IntegerModularManager) CleanupConstructedMembers(cursor *uint8) error {
	if m == nil || !m.ValidOwner() || cursor == nil {
		return ErrModularOwner
	}
	fields := [7]*Int{&m.P, &m.Lower, &m.Upper, &m.InvTmp1, &m.InvTmp2, &m.InvTmp3, &m.DivTmp}
	for *cursor < uint8(len(fields)) {
		if e := m.manager.Del(fields[*cursor]); e != nil {
			return e
		}
		*cursor++
	}
	return nil
}

// FinishSourceLifetime refunds the Go header after the containing caller has
// completed those source member destructors, without invoking them again.
func (m *IntegerModularManager) FinishSourceLifetime() error {
	if m == nil || !m.ValidOwner() {
		return ErrModularOwner
	}
	m.manager.BudgetField.Release(m.Header)
	m.Header = 0
	m.closed = true
	return nil
}
func (m *IntegerModularManager) FinalizeSourceMember(index uint32) (bool, error) {
	if m == nil || !m.ValidOwner() {
		return false, ErrModularOwner
	}
	fields := [7]*Int{&m.P, &m.Lower, &m.Upper, &m.InvTmp1, &m.InvTmp2, &m.InvTmp3, &m.DivTmp}
	if index >= uint32(len(fields)) {
		return false, ErrNumericOwner
	}
	return SourceNumeralDel(m.manager, fields[index])
}
func (m *IntegerModularManager) CleanupRawMember(index uint32) error {
	if m == nil || !m.ValidOwner() {
		return ErrModularOwner
	}
	fields := [7]*Int{&m.P, &m.Lower, &m.Upper, &m.InvTmp1, &m.InvTmp2, &m.InvTmp3, &m.DivTmp}
	if index >= uint32(len(fields)) {
		return ErrNumericOwner
	}
	return m.manager.CleanupRawNumeral(fields[index])
}
