package polynomial

// Source: src/math/polynomial/polynomial.cpp:3186-3205,7359-7361;
// src/math/polynomial/polynomial.h:924.

// Vars resets xs, then publishes distinct variables in first-encounter order.
// A source body failure does not execute the final found-variable reset loop.
// xs owns its vector; the manager owns its persistent bool vector. There are
// no local source NUM/ref declarations or callee owning scopes.
func (m *MultivariatePolynomialManager) Vars(p Polynomial, xs *PolynomialVariableDegrees) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.varsCore(p, xs)
}

func (c *MultivariatePolynomialCore) varsCore(p Polynomial, xs *PolynomialVariableDegrees) error {
	if e := xs.open(); e != nil {
		return e
	}
	if xs.runtime != c.memory {
		return ErrPolynomialOwner
	}
	c.collectingVariables = true
	xs.collecting = true
	defer func() { xs.collecting = false; c.collectingVariables = false }()
	if e := variableReset(xs); e != nil {
		return e
	}
	variables, e := c.monomials.NumVars()
	if e != nil {
		return e
	}
	found := &c.substitution.found
	if e = found.reserve(variables); e != nil {
		return e
	}
	n, e := c.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return e
		}
		size, e := c.monomials.sizeCore(term)
		if e != nil {
			return e
		}
		for j := uint32(0); j < size; j++ {
			x, e := c.monomials.varCore(term, j)
			if e != nil {
				return e
			}
			seen, e := found.at(x)
			if e != nil {
				return e
			}
			if !seen {
				if e = found.put(x, true); e != nil {
					return e
				}
				if e = variablePush(xs, x); e != nil {
					return e
				}
			}
		}
	}
	n, e = xs.size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		x, e := variableAt(xs, i)
		if e != nil {
			return e
		}
		if e = found.put(x, false); e != nil {
			return e
		}
	}
	return nil
}
