package polynomial

// Source: src/math/polynomial/polynomial.cpp:2464-2480,7026-7032.

import uhash "github.com/glycerine/g3/pkg/util/hash"

// Hash computes the source polynomial hash after lexicographically sorting its
// terms. The source cache uses this value to bucket structurally equal values.
func (m *MultivariatePolynomialManager) Hash(p Polynomial) (uint32, error) {
	return m.hashWithLexReceipt(p, nil)
}
func (m *MultivariatePolynomialManager) hashWithLexReceipt(p Polynomial, r *polynomialLexSortInvocationReceipt) (uint32, error) {
	if e := m.ready(); e != nil {
		return 0, e
	}
	size, e := m.core.readWord(p, 8)
	if e != nil {
		return 0, e
	}
	if size == 0 {
		return 31, nil
	}
	if e = m.lexSortWithReceipt(p, nil, r); e != nil {
		return 0, e
	}
	size, e = m.core.readWord(p, 8)
	if e != nil {
		return 0, e
	}
	var cause error
	h := uhash.Composite(17, size, func(i uint32) uint32 {
		if cause != nil {
			return 0
		}
		var child uint32
		child, cause = m.polynomialHashChild(p, i)
		return child
	})
	if cause != nil {
		return 0, cause
	}
	return h, nil
}

func (m *MultivariatePolynomialManager) polynomialHashChild(p Polynomial, i uint32) (uint32, error) {
	mono, e := m.core.monomialCore(p, i)
	if e != nil {
		return 0, e
	}
	monomialHash, e := m.core.monomials.Hash(mono)
	if e != nil {
		return 0, e
	}
	coefficient, e := m.core.coefficientCore(p, i)
	if e != nil {
		return 0, e
	}
	valueHash, e := m.core.modular.Hash(&coefficient)
	if e != nil {
		return 0, e
	}
	return uhash.UintPair(monomialHash, valueHash), nil
}
