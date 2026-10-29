package symbol

// Source: src/util/symbol.cpp:50-74.

func (m *Manager) InternBorrowedSymbolCString(s Symbol) (Symbol, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.checkPlain(s); e != nil {
		return Symbol{}, e
	}
	if s.IsNumerical() {
		if e := m.bareStringNumericalAssertion(); e != nil {
			return Symbol{}, e
		}
		// symbol.h:96 numerical bare_str is the actual empty string in release.
		return m.intern("")
	}
	if s.Value == 0 {
		return Symbol{}, ErrInvalid
	} // actual null char-pointer guard
	return s, nil
}
