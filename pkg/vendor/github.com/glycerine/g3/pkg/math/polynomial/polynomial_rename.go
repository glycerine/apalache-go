package polynomial

// Source: src/math/polynomial/polynomial.cpp:1464-1475,6070-6097,7356-7357.

func (m *MultivariatePolynomialCore) makeFirstMaximal(p Polynomial) error {
	n, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	if n <= 1 {
		return nil
	}
	best := uint32(0)
	for i := uint32(1); i < n; i++ {
		a, e := m.monomialCore(p, i)
		if e != nil {
			return e
		}
		b, e := m.monomialCore(p, best)
		if e != nil {
			return e
		}
		greater, e := m.firstGreater(a, b)
		if e != nil {
			return e
		}
		if greater {
			best = i
		}
	}
	if e = m.swapTerms(p, 0, best); e != nil {
		return e
	}
	flags, e := m.readWord(p, 4)
	if e != nil {
		return e
	}
	return m.writeWord(p, 4, flags&0x7fffffff)
}

// Rename traverses the literal source pointer directory after the actual
// monomial manager operation. This is a mutating source algorithm, not owning
// discovery or cleanup of caller values.
func (m *MultivariatePolynomialManager) Rename(xs []uint32) error {
	if e := m.ready(); e != nil {
		return e
	}
	if e := m.traceRename(xs, true); e != nil {
		return e
	}
	if e := m.core.monomials.Rename(xs); e != nil {
		return e
	}
	for _, p := range m.core.polynomials {
		if p == (Polynomial{}) {
			continue
		}
		if e := m.core.makeFirstMaximal(p); e != nil {
			return e
		}
		if e := polynomialRenameSourceAssert(6089, "p->size() <= 1 || !p->lex_sorted()", func() (bool, error) {
			size, e := m.core.readWord(p, 8)
			if e != nil {
				return false, e
			}
			if size <= 1 {
				return true, nil
			}
			flags, e := m.core.readWord(p, 4)
			if e != nil {
				return false, e
			}
			return flags>>31 == 0, nil
		}); e != nil {
			return e
		}
	}
	return m.traceRename(xs, false)
}
