package numeral

func (m *IntegerModularManager) finishPowerSource() error {
	if !m.PowerPending {
		return nil
	}
	if _, e := sourceNumeralDel(m.manager, &m.PowerTemporary); e != nil {
		return e
	}
	m.PowerTemporary = Int{}
	m.PowerPending = false
	return nil
}

// CleanupPowerTemporary owns a raw local retained after a stopped power body
// or source end-label deletion. It does not replay that source deletion.
func (m *IntegerModularManager) CleanupPowerTemporary() error {
	if e := m.check(); e != nil {
		return e
	}
	if !m.PowerPending {
		return nil
	}
	if e := m.manager.delWithModularOwner(&m.PowerTemporary, m); e != nil {
		return e
	}
	m.PowerTemporary = Int{}
	m.PowerPending = false
	return nil
}
