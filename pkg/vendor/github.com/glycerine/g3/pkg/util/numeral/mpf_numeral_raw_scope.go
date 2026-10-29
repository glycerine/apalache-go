package numeral

func (m *ArbitraryFloatNumeralManager) beginRawLocal() error {
	if m.rawPending {
		return ErrArbitraryFloatNumeralRawPending
	}
	m.rawLocal = ArbitraryFloat{}
	m.rawPending = true
	return nil
}

func (m *ArbitraryFloatNumeralManager) finishRawLocalSource() error {
	if !m.rawPending {
		return nil
	}
	if e := m.core.valueGuard(&m.rawLocal); e != nil {
		return e
	}
	if _, e := sourceNumeralDel(m.core.integer, &m.rawLocal.significand); e != nil {
		return e
	}
	m.rawLocal = ArbitraryFloat{}
	m.rawPending = false
	return nil
}

// CleanupRawLocal owns the unfinished reciprocal or power local after a
// stopped source body or deletion. It does not replay source deletion.
func (m *ArbitraryFloatNumeralManager) CleanupRawLocal() error {
	if e := m.guard(); e != nil {
		return e
	}
	if !m.rawPending {
		return nil
	}
	if e := m.core.CleanupValue(&m.rawLocal); e != nil {
		return e
	}
	m.rawLocal = ArbitraryFloat{}
	m.rawPending = false
	return nil
}
