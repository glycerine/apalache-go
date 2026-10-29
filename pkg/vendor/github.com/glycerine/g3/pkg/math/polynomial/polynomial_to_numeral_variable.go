package polynomial

// Source: src/math/polynomial/upolynomial.h:426-450.

// ToNumeralVectorInto converts a univariate polynomial into the concrete dense
// owner used by the caller. The two owners must share the actual integer
// service; no coefficient-store alias or intermediate polynomial is created.
func (m *MultivariatePolynomialManager) ToNumeralVectorInto(p Polynomial, outputManager *UPolynomialManager, output UPolynomial) error {
	if e := m.ready(); e != nil {
		return e
	}
	if outputManager == nil || outputManager.config.Integer != m.core.integer {
		return ErrUPolynomialOwner
	}
	d, e := outputManager.descriptor(output)
	if e != nil {
		return e
	}
	return m.toNumeralVectorIntoSource(p, 0, false, outputManager, &d.state, nil)
}

// UnivariateManager returns the actual dense manager owned by this manager.
// The caller must keep its owner alive while using the borrowed manager.
func (m *MultivariatePolynomialManager) UnivariateManager() (*UPolynomialManager, error) {
	if e := m.ready(); e != nil {
		return nil, e
	}
	return m.upm, nil
}

// ToNumeralVectorVariable keeps the constant and pure powers of variable.
// output belongs to the actual borrowed UnivariateManager; it remains caller-owned.
func (m *MultivariatePolynomialManager) ToNumeralVectorVariable(p Polynomial, variable uint32, output UPolynomial) error {
	if e := m.ready(); e != nil {
		return e
	}
	d, e := m.upm.descriptor(output)
	if e != nil {
		return e
	}
	return m.toNumeralVectorVariableWithReceipt(p, variable, &d.state, nil)
}

// CleanupNumeralVectorConversion acknowledges only the actual current conversion.
// It never resets or deletes the caller's output polynomial.
func (m *MultivariatePolynomialManager) CleanupNumeralVectorConversion() error {
	if m == nil || m.self != m || m.core == nil || m.core.self != m.core {
		return ErrPolynomialOwner
	}
	r := m.factorNumeralVector.direct
	if e := m.cleanupFactorNumeralVector(&r, false); e != nil {
		return e
	}
	return m.cleanupFactorNumeralVector(&r, true)
}
