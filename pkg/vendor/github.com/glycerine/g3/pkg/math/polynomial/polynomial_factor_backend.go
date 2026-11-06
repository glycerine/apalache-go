package polynomial

// Source: src/math/polynomial/polynomial.cpp:6683-6725;
// src/util/prime_generator.cpp:99-107.
// The Go service borrows the explicit config selecting the actual generator.

func (m *MultivariatePolynomialManager) factorizationService() (*UPolynomialFactorizationService, error) {
	if e := m.ready(); e != nil {
		return nil, e
	}
	if m.factorService != nil {
		if e := m.factorService.checkOwner(); e != nil {
			return nil, e
		}
		return m.factorService, nil
	}
	s, e := NewUPolynomialFactorizationService(m.upm, m.factorConfig)
	if e != nil {
		return nil, e
	}
	m.factorService = s
	return s, nil
}
