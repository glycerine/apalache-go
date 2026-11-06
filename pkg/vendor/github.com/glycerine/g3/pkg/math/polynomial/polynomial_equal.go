package polynomial

// Source: src/math/polynomial/polynomial.cpp:449-505,4631-4655.

// queryPositions is the ONE concrete source m_m2pos member, not a private map.
func (m *MultivariatePolynomialCore) queryPositionsReady() {
	if m.queryPositions.self == nil {
		m.queryPositions.self = &m.queryPositions
		m.queryPositions.manager = m.monomials
	}
}
func (m *MultivariatePolynomialCore) setPolynomialPositions(p Polynomial) error {
	n, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return e
		}
		if e = m.queryPositions.Set(term, i); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) resetPolynomialPositions(p Polynomial) error {
	n, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return e
		}
		if e = m.queryPositions.Reset(term); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) Equal(a, b Polynomial) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if a == b {
		return true, nil
	}
	na, e := m.readWord(a, 8)
	if e != nil {
		return false, e
	}
	nb, e := m.readWord(b, 8)
	if e != nil {
		return false, e
	}
	if na != nb {
		return false, nil
	}
	if na == 0 {
		return true, nil
	}
	va, e := m.MaxVariable(a)
	if e != nil {
		return false, e
	}
	vb, e := m.MaxVariable(b)
	if e != nil {
		return false, e
	}
	if va != vb {
		return false, nil
	}
	m.queryPositionsReady()
	if e = m.setPolynomialPositions(a); e != nil {
		return false, e
	}
	for i := uint32(0); i < nb; i++ {
		term, e := m.monomialCore(b, i)
		if e != nil {
			return false, e
		}
		pos, e := m.queryPositions.Get(term)
		if e != nil {
			return false, e
		}
		different := pos == ^uint32(0)
		if !different {
			ca, e := m.coefficientCore(a, pos)
			if e != nil {
				return false, e
			}
			cb, e := m.coefficientCore(b, i)
			if e != nil {
				return false, e
			}
			same, e := m.modular.Equal(&ca, &cb)
			if e != nil {
				return false, e
			}
			different = !same
		}
		if different {
			if e = m.resetPolynomialPositions(a); e != nil {
				return false, e
			}
			return false, nil
		}
	}
	if e = m.resetPolynomialPositions(a); e != nil {
		return false, e
	}
	return true, nil
}
