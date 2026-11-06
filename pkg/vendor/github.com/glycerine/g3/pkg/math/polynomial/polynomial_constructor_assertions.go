package polynomial

// Source: src/math/polynomial/polynomial.cpp:2531,2539,2558,2560,2569,2605-2606,2626,2631.

// The concrete constructor helpers share the cell's source assertion transport;
// each predicate remains lazy and each source statement keeps its own gate.
func (m *MultivariatePolynomialCore) linearConstructorEmptyAssertions() error {
	if e := m.cellSourceAssert(2605, "m_tmp_linear_as.empty()", func() (bool, error) {
		n, e := m.linearTerms.size(m)
		return n == 0, e
	}); e != nil {
		return e
	}
	return m.cellSourceAssert(2606, "m_tmp_linear_ms.empty()", func() (bool, error) {
		return len(m.linearMonomials) == 0, nil
	})
}
