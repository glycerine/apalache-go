package ast

// Source: src/ast/ast.h:1507-1508,1600; src/ast/ast.cpp:1798-1844.

// EnableDebugRefCount performs the actual source one-way flag publication.
// Node deletion consults this member at each real recycle/info/free-ID action.
func (m *ASTManager) EnableDebugRefCount() error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	m.DebugRefCount = true
	return nil
}

func (m *ASTManager) EnableIntRealCoercions(enabled bool) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	m.coercions = enabled
	return nil
}

func (m *ASTManager) IntRealCoercions() (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.coercions, nil
}
