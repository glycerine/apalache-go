package polynomial

// Source: src/math/polynomial/polynomial.cpp:2117-2139,6898-6927.

// Both source loops read VAR, then the mapping, and only then a reached DEG.
func (m *MultivariatePolynomialCore) monomialSurvivesModD(term Monomial, d *PolynomialVariableDegrees) (bool, error) {
	n, e := m.monomials.sizeCore(term)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		x, e := m.monomials.varCore(term, i)
		if e != nil {
			return false, e
		}
		bound, e := d.Degree(x)
		if e != nil {
			return false, e
		}
		if bound == 0 {
			continue
		}
		degree, e := m.monomials.degreeAtCore(term, i)
		if e != nil {
			return false, e
		}
		if degree >= bound {
			return false, nil
		}
	}
	return true, nil
}
func (b *polynomialSOM) modD(d *PolynomialVariableDegrees) error {
	m := b.owner
	n, e := b.ms.size(m)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		if polynomialSourceDebug {
			size, e := b.as.size(m)
			if e != nil {
				return e
			}
			if i >= size {
				return ErrPolynomialAssertion
			}
		}
		a, e := b.as.coefficient(m, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(&a)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		term, e := b.ms.monomial(m, i)
		if e != nil {
			return e
		}
		survive, e := m.monomialSurvivesModD(term, d)
		if e != nil {
			return e
		}
		if !survive {
			e = m.modular.Reset(&a)
			if w := b.as.putCoefficient(m, i, a); w != nil {
				return w
			}
			if e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) ModD(p Polynomial, d *PolynomialVariableDegrees) (Polynomial, error) {
	return m.modDWithReceipt(p, d, nil)
}
func (m *MultivariatePolynomialCore) modDWithReceipt(p Polynomial, d *PolynomialVariableDegrees, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	constant, e := m.IsConstant(p)
	if e != nil {
		return Polynomial{}, e
	}
	if constant {
		return p, nil
	}
	if e = m.cheap.reset(); e != nil {
		return Polynomial{}, e
	}
	if receipt != nil {
		if e = m.cheap.acquireArithmeticInvocation(receipt); e != nil {
			return Polynomial{}, e
		}
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		survive, e := m.monomialSurvivesModD(term, d)
		if e != nil {
			return Polynomial{}, e
		}
		if survive {
			a, e := m.coefficientCore(p, i)
			if e != nil {
				return Polynomial{}, e
			}
			// Source p->m(i) is reread at the actual R.add call.
			term, e = m.monomialCore(p, i)
			if e != nil {
				return Polynomial{}, e
			}
			if e = (m.cheap).addWithArithmeticReceipt(&a, term, receipt); e != nil {
				return Polynomial{}, e
			}
		}
	}
	return m.cheap.mkWithArithmeticReceipt(false, receipt)
}
func (m *MultivariatePolynomialCore) ExactPseudoDivideModD(p, q Polynomial, x uint32, d *PolynomialVariableDegrees, Q, R *PolynomialRef) (err error) {
	if Q == nil {
		return ErrPolynomialOwner
	}
	if e := m.pseudoEntry(Q, R); e != nil {
		return e
	}
	defer m.pseudoFinish(&err)
	var exponent uint32
	return m.pseudoCore(p, q, x, &exponent, &Q.root, &R.root, true, true, true, d)
}
