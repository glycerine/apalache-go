package ast

// DependencyAddressText prints the concrete dependency allocation address used
// by source ostream diagnostics. The value is borrowed and owns no reference.
func (m *ASTManager) DependencyAddressText(h ASTDependency) (string, error) {
	if err := m.lock(); err != nil {
		return "", err
	}
	defer m.mu.Unlock()
	r, err := m.dependencies.valid(h)
	if err != nil {
		return "", err
	}
	if h.IsNull() {
		return "0", nil
	}
	return m.dependencies.allocator.AddressText(r)
}
