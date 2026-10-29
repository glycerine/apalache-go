package polynomial

// Source: src/math/polynomial/polynomial.cpp:6101-6176,7336-7338;
// src/math/polynomial/polynomial.h:884-913; src/util/lbool.h.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// PolynomialSign is the source three-valued sign. Unknown also represents
// disagreement between term signs; this query does not evaluate cancellation.

type PolynomialSign int32

const (
	PolynomialSignNegative PolynomialSign = -1
	PolynomialSignUnknown  PolynomialSign = 0
	PolynomialSignPositive PolynomialSign = 1
)

func (m *MultivariatePolynomialCore) termSign(term Monomial, coefficient *unumeral.Int, signs []PolynomialSign) (PolynomialSign, error) {
	n, e := m.monomials.sizeCore(term)
	if e != nil {
		return PolynomialSignUnknown, e
	}
	positive, e := m.modular.IsPos(coefficient)
	if e != nil {
		return PolynomialSignUnknown, e
	}
	sign := PolynomialSignNegative
	if positive {
		sign = PolynomialSignPositive
	}
	for i := uint32(0); i < n; i++ {
		variable, e := m.monomials.varCore(term, i)
		if e != nil {
			return PolynomialSignUnknown, e
		}
		degree, e := m.monomials.degreeAtCore(term, i)
		if e != nil {
			return PolynomialSignUnknown, e
		}
		other := PolynomialSignUnknown
		if uint64(variable) < uint64(len(signs)) {
			other = signs[variable]
		}
		if other == PolynomialSignUnknown {
			return PolynomialSignUnknown, nil
		}
		if degree%2 == 1 && other == PolynomialSignNegative {
			if sign == PolynomialSignPositive {
				sign = PolynomialSignNegative
			} else {
				sign = PolynomialSignPositive
			}
		}
	}
	return sign, nil
}

// Sign follows the source term-consensus check, including unknown variables
// even for even exponents. No numeric temporaries or owning references arise.
func (m *MultivariatePolynomialManager) Sign(p Polynomial, signs []PolynomialSign) (PolynomialSign, error) {
	if e := m.ready(); e != nil {
		return PolynomialSignUnknown, e
	}
	c := m.core
	n, e := c.readWord(p, 8)
	if e != nil {
		return PolynomialSignUnknown, e
	}
	if n == 0 {
		return PolynomialSignUnknown, nil
	}
	term, e := c.monomialCore(p, 0)
	if e != nil {
		return PolynomialSignUnknown, e
	}
	coefficient, e := c.coefficientCore(p, 0)
	if e != nil {
		return PolynomialSignUnknown, e
	}
	sign, e := c.termSign(term, &coefficient, signs)
	if e != nil {
		return PolynomialSignUnknown, e
	}
	for i := uint32(1); sign != PolynomialSignUnknown && i < n; i++ {
		term, e = c.monomialCore(p, i)
		if e != nil {
			return PolynomialSignUnknown, e
		}
		coefficient, e = c.coefficientCore(p, i)
		if e != nil {
			return PolynomialSignUnknown, e
		}
		other, e := c.termSign(term, &coefficient, signs)
		if e != nil {
			return PolynomialSignUnknown, e
		}
		if other != sign {
			return PolynomialSignUnknown, nil
		}
	}
	return sign, nil
}

// The source's non-strict checks use strictly signed coefficients; only the
// absence of a constant term distinguishes them from the strict checks.
func (m *MultivariatePolynomialManager) signedEvenTerms(p Polynomial, positive, requireUnit bool) (bool, error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	c := m.core
	n, e := c.readWord(p, 8)
	if e != nil {
		return false, e
	}
	foundUnit := false
	for i := uint32(0); i < n; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return false, e
		}
		even, e := c.monomials.IsPowerOfTwo(term)
		if e != nil {
			return false, e
		}
		if !even {
			return false, nil
		}
		if requireUnit && term == c.monomials.unit {
			foundUnit = true
		}
		coefficient, e := c.coefficientCore(p, i)
		if e != nil {
			return false, e
		}
		var signed bool
		if positive {
			signed, e = c.modular.IsPos(&coefficient)
		} else {
			signed, e = c.modular.IsNeg(&coefficient)
		}
		if e != nil {
			return false, e
		}
		if !signed {
			return false, nil
		}
	}
	if requireUnit {
		return foundUnit, nil
	}
	return true, nil
}
func (m *MultivariatePolynomialManager) IsPos(p Polynomial) (bool, error) {
	return m.signedEvenTerms(p, true, true)
}
func (m *MultivariatePolynomialManager) IsNeg(p Polynomial) (bool, error) {
	return m.signedEvenTerms(p, false, true)
}
func (m *MultivariatePolynomialManager) IsNonPos(p Polynomial) (bool, error) {
	return m.signedEvenTerms(p, false, false)
}
func (m *MultivariatePolynomialManager) IsNonNeg(p Polynomial) (bool, error) {
	return m.signedEvenTerms(p, true, false)
}
