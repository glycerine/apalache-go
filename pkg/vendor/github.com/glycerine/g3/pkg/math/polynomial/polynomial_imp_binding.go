package polynomial

// Source binding: src/math/polynomial/polynomial.cpp:2303-2370,6683-6830.

// ArithmeticCore borrows this manager's actual arithmetic implementation for
// Go callers constructing its inputs. This binding adds no second owner and
// does not represent a named C++ getter.
func (m *MultivariatePolynomialManager) ArithmeticCore() (*MultivariatePolynomialCore, error) {
	if e := m.ready(); e != nil {
		return nil, e
	}
	return m.core, nil
}
