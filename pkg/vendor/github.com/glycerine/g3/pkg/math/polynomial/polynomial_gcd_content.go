package polynomial

// Source: src/math/polynomial/polynomial.cpp:2503-2558.

import (
	"math"
	"math/big"
)

func (m *MultivariatePolynomialCore) polynomialContent(p Polynomial, relation GCDSimplifyRelation) (*big.Int, []*big.Int, []Monomial, error) {
	info, err := m.Info(p)
	if err != nil {
		return nil, nil, nil, err
	}
	z, err := m.IntegerManager()
	if err != nil {
		return nil, nil, nil, err
	}
	g := new(big.Int)
	values := make([]*big.Int, info.Size)
	monomials := make([]Monomial, info.Size)
	slow := false
	for i := uint32(0); i < info.Size; i++ {
		coefficient, cause := m.Coefficient(p, i)
		if cause != nil {
			return nil, nil, nil, cause
		}
		if !coefficient.IsSmall() || coefficient.NativeScalar() == math.MinInt32 {
			slow = true
		}
		text, cause := z.String(&coefficient)
		if cause != nil {
			return nil, nil, nil, cause
		}
		value, ok := new(big.Int).SetString(text, 10)
		if !ok {
			return nil, nil, nil, ErrPolynomialOwner
		}
		values[i] = value
		monomials[i], cause = m.TermMonomial(p, i)
		if cause != nil {
			return nil, nil, nil, cause
		}
	}
	for i := range values {
		constant, cause := m.gcdSimplifyConstant(monomials[i])
		if cause != nil {
			return nil, nil, nil, cause
		}
		magnitude := new(big.Int).Abs(new(big.Int).Set(values[i]))
		// The source fast path skips the strict-relation constant before
		// checking units. Its arbitrary-precision fallback checks units first.
		if slow && magnitude.Cmp(big.NewInt(1)) == 0 {
			return big.NewInt(1), values, monomials, nil
		}
		if relation != GCDSimplifyEquality && constant {
			continue
		}
		if magnitude.Cmp(big.NewInt(1)) == 0 {
			return big.NewInt(1), values, monomials, nil
		}
		g.GCD(nil, nil, g, magnitude)
	}
	return g, values, monomials, nil
}

func (m *MultivariatePolynomialCore) gcdSimplifyConstant(term Monomial) (bool, error) {
	mm, err := m.MonomialManager()
	if err != nil {
		return false, err
	}
	info, err := mm.Info(term)
	return info.Size == 0, err
}
