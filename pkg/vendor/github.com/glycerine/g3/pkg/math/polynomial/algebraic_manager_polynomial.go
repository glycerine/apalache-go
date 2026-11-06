package polynomial

// Source: src/api/api_context.h pmanager/algebraic manager sharing.  The
// returned manager is borrowed from this algebraic owner.
func (m *AlgebraicManager) PolynomialManager() (*MultivariatePolynomialManager, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	return m.polynomial, nil
}
