package ast

// Source: src/ast/ast.h:1455,1549-1550.

// Canceled increments the manager's source resource count before testing the
// cancellation flag or step limit. Suspension permits work without hiding the
// count increment, as in reslimit::inc.
func (m *ASTManager) Canceled() (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	return !m.Limit.Inc(), nil
}
