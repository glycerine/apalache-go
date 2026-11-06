package polynomial

// Source: src/math/polynomial/polynomial.cpp:1885-2156.

import "github.com/glycerine/g3/pkg/arena"

// Native monomial handles; the enclosing SOM owns their actual references.
type polynomialMonomials []Monomial

func (v *polynomialMonomials) size(m *MultivariatePolynomialCore) (uint32, error) {
	return uint32(len(*v)), nil
}
func (v *polynomialMonomials) monomial(m *MultivariatePolynomialCore, i uint32) (Monomial, error) {
	if uint64(i) >= uint64(len(*v)) {
		return Monomial{}, arena.ErrBounds
	}
	return (*v)[i], nil
}
func (v *polynomialMonomials) putMonomial(m *MultivariatePolynomialCore, i uint32, a Monomial) error {
	if uint64(i) >= uint64(len(*v)) {
		return arena.ErrBounds
	}
	(*v)[i] = a
	return nil
}
func (v *polynomialMonomials) publishSize(m *MultivariatePolynomialCore, n uint32) error {
	if uint64(n) > uint64(len(*v)) {
		return arena.ErrBounds
	}
	clear((*v)[n:])
	*v = (*v)[:n]
	return nil
}
func (v *polynomialMonomials) reset(m *MultivariatePolynomialCore) error {
	clear(*v)
	*v = (*v)[:0]
	return nil
}
func (v *polynomialMonomials) close(m *MultivariatePolynomialCore) error { *v = nil; return nil }
func (v *polynomialMonomials) appendMonomial(m *MultivariatePolynomialCore, a Monomial) error {
	return v.appendMonomialWithAppendScope(m, a, nil, false)
}
func (v *polynomialMonomials) appendMonomialWithAppendScope(m *MultivariatePolynomialCore, a Monomial, scope *polynomialCheapAppendScope, owned bool) error {
	i := uint32(len(*v))
	*v = append(*v, a)
	if scope != nil {
		scope.buffer.monomialTransferred[i] = 0
		if owned {
			scope.buffer.monomialOwners[i] = scope.invocation
			scope.monomialClaims++
			scope.monomials = i + 1
			scope.pending = Monomial{}
		} else {
			scope.buffer.monomialOwners[i] = 0
			if scope.retirePhase == 1 || scope.retirePhase == 2 {
				scope.retireMs = i + 1
			}
			scope.foreignMixed = true
		}
	}
	return nil
}
