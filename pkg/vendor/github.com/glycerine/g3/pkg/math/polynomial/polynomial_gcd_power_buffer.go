package polynomial

// Source: src/math/polynomial/polynomial.cpp:3207-3256,4352-4506.

import (
	"sort"

	"github.com/glycerine/g3/pkg/arena"
)

// GCD variable/degree pairs use native Go storage. The enclosing invocation
// controls the slice lifetime; pairs own no numeral or monomial references.
type polynomialGCDPowers []MonomialPower

func (b *polynomialGCDPowers) word(i, field uint32) (uint32, error) {
	if uint64(i) >= uint64(len(*b)) {
		if polynomialSourceDebug {
			return 0, ErrPolynomialAssertion
		}
		return 0, arena.ErrBounds
	}
	if field == 0 {
		return (*b)[i].Var, nil
	}
	return (*b)[i].Degree, nil
}
func (b *polynomialGCDPowers) setWord(i, field, x uint32) error {
	if uint64(i) >= uint64(len(*b)) {
		if polynomialSourceDebug {
			return ErrPolynomialAssertion
		}
		return arena.ErrBounds
	}
	if field == 0 {
		(*b)[i].Var = x
	} else {
		(*b)[i].Degree = x
	}
	return nil
}
func (b *polynomialGCDPowers) push(x, k uint32) error {
	if uint64(len(*b)) == uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	*b = append(*b, MonomialPower{x, k})
	return nil
}
func (b *polynomialGCDPowers) sortVariables() {
	sort.Slice(*b, func(i, j int) bool { return (*b)[i].Var < (*b)[j].Var })
}
func (b *polynomialGCDPowers) sortDegrees() {
	sort.Slice(*b, func(i, j int) bool { return (*b)[i].Degree < (*b)[j].Degree })
}

func (m *MultivariatePolynomialManager) variableDegrees(p Polynomial, pws *polynomialGCDPowers, max bool) error {
	c := m.core
	*pws = (*pws)[:0]
	variables, e := c.monomials.NumVars()
	if e != nil {
		return e
	}
	for uint64(len(m.varDegrees)) < uint64(variables) {
		m.varDegrees = append(m.varDegrees, ^uint32(0))
	}
	size, e := c.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return e
		}
		count, e := c.monomials.sizeCore(term)
		if e != nil {
			return e
		}
		for j := uint32(0); j < count; j++ {
			x, e := c.monomials.varCore(term, j)
			if e != nil {
				return e
			}
			k, e := c.monomials.degreeAtCore(term, j)
			if e != nil {
				return e
			}
			if uint64(x) >= uint64(len(m.varDegrees)) {
				return arena.ErrBounds
			}
			pos := m.varDegrees[x]
			if pos == ^uint32(0) {
				m.varDegrees[x] = uint32(len(*pws))
				if e = pws.push(x, k); e != nil {
					return e
				}
			} else {
				old, e := pws.word(pos, 1)
				if e != nil {
					return e
				}
				if max && k > old || !max && k < old {
					if e = pws.setWord(pos, 1, k); e != nil {
						return e
					}
				}
			}
		}
	}
	for i := uint32(0); i < uint32(len(*pws)); i++ {
		if e := polynomialGCDPowerSourceAssert(3244, "var2pos[pws[i].get_var()] != UINT_MAX", func() (bool, error) {
			x, xerr := pws.word(i, 0)
			if xerr != nil {
				return false, xerr
			}
			if uint64(x) >= uint64(len(m.varDegrees)) {
				return false, arena.ErrBounds
			}
			return m.varDegrees[x] != ^uint32(0), nil
		}); e != nil {
			return e
		}
		x, e := pws.word(i, 0)
		if e != nil {
			return e
		}
		m.varDegrees[x] = ^uint32(0)
	}
	if polynomialSourceDebug {
		for i := uint32(0); i < uint32(len(*pws)); i++ {
			for j := i + 1; j < uint32(len(*pws)); j++ {
				if e := polynomialGCDPowerSourceAssert(3251, "pws[i].first != pws[j].first", func() (bool, error) {
					x, xerr := pws.word(i, 0)
					if xerr != nil {
						return false, xerr
					}
					y, xerr := pws.word(j, 0)
					return x != y, xerr
				}); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
