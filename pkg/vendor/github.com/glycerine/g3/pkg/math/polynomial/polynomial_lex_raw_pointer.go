package polynomial

// Source: src/math/polynomial/polynomial.cpp:1315-1366,1401-1420;
// src/util/permutation.h:56-91.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// These concrete call-local pointers borrow actual polynomial payload fields.
// Their ObjectRef is captured, not looked up as a replacement at each access.
// The checked owner/init-prefix guards qualify native raw-pointer lifetime;
// logical SIZE never participates in the reached raw-pointer word access.
type polynomialCoefficientPointer struct {
	core  *MultivariatePolynomialCore
	value Polynomial
	raw   umemory.ObjectRef
	start uint32
}
type polynomialMonomialPointer struct {
	core  *MultivariatePolynomialCore
	value Polynomial
	raw   umemory.ObjectRef
	start uint32
	base  uint64
}

func (c *MultivariatePolynomialCore) captureCoefficientPointer(p Polynomial, start uint32) (polynomialCoefficientPointer, error) {
	l, e := c.polynomialLocation(p)
	if e != nil {
		return polynomialCoefficientPointer{}, e
	}
	return polynomialCoefficientPointer{core: c, value: p, raw: l.raw, start: start}, nil
}
func (c *MultivariatePolynomialCore) captureMonomialPointer(p Polynomial, start uint32) (polynomialMonomialPointer, error) {
	l, e := c.polynomialLocation(p)
	if e != nil {
		return polynomialMonomialPointer{}, e
	}
	return polynomialMonomialPointer{core: c, value: p, raw: l.raw, start: start, base: l.monomialBase}, nil
}
func polynomialCapturedLocation(c *MultivariatePolynomialCore, p Polynomial, raw umemory.ObjectRef) (*polynomialLocation, error) {
	if c == nil {
		return nil, ErrPolynomialOwner
	}
	l, e := c.polynomialLocation(p)
	if e != nil {
		return nil, e
	}
	if l.raw != raw {
		return nil, ErrPolynomialOwner
	}
	return l, nil
}
func (v polynomialCoefficientPointer) address(i uint32) (uint64, error) {
	l, e := polynomialCapturedLocation(v.core, v.value, v.raw)
	if e != nil {
		return 0, e
	}
	j := uint64(v.start) + uint64(i)
	if j >= uint64(l.initializedCoefficients) {
		return 0, ErrPolynomialPartial
	}
	return 12 + j*12, nil
}
func (v polynomialCoefficientPointer) read(i uint32) (unumeral.Int, error) {
	o, e := v.address(i)
	if e != nil {
		return unumeral.Int{}, e
	}
	a := v.core.monomials.allocator
	val, e := a.ReadUint32(v.raw, o)
	if e != nil {
		return unumeral.Int{}, e
	}
	lo, e := a.ReadUint32(v.raw, o+4)
	if e != nil {
		return unumeral.Int{}, e
	}
	hi, e := a.ReadUint32(v.raw, o+8)
	return unumeral.IntFromNativeWords([3]uint32{uint32(val), uint32(uint64(lo) | uint64(hi)<<32), uint32(uint64(uint64(lo)|uint64(hi)<<32) >> 32)}), e
}
func (v polynomialCoefficientPointer) write(i uint32, x unumeral.Int) error {
	o, e := v.address(i)
	if e != nil {
		return e
	}
	a := v.core.monomials.allocator
	if e = a.WriteUint32(v.raw, o, uint32(x.NativeScalar())); e != nil {
		return e
	}
	if e = a.WriteUint32(v.raw, o+4, uint32(x.NativeCellToken())); e != nil {
		return e
	}
	return a.WriteUint32(v.raw, o+8, uint32(uint64(x.NativeCellToken())>>32))
}
func (v polynomialMonomialPointer) address(i uint32) (uint64, error) {
	l, e := polynomialCapturedLocation(v.core, v.value, v.raw)
	if e != nil {
		return 0, e
	}
	j := uint64(v.start) + uint64(i)
	if j >= uint64(l.initializedMonomials) {
		return 0, ErrPolynomialPartial
	}
	return v.base + j*16, nil
}
func (v polynomialMonomialPointer) read(i uint32) (Monomial, error) {
	o, e := v.address(i)
	if e != nil {
		return Monomial{}, e
	}
	a := v.core.monomials.allocator
	lo, e := a.ReadUint32(v.raw, o)
	if e != nil {
		return Monomial{}, e
	}
	hi, e := a.ReadUint32(v.raw, o+4)
	if e != nil {
		return Monomial{}, e
	}
	s, e := a.ReadUint32(v.raw, o+8)
	if e != nil {
		return Monomial{}, e
	}
	g, e := a.ReadUint32(v.raw, o+12)
	return Monomial{owner: uint64(lo) | uint64(hi)<<32, slot: s, generation: g}, e
}
func (v polynomialMonomialPointer) write(i uint32, x Monomial) error {
	o, e := v.address(i)
	if e != nil {
		return e
	}
	a := v.core.monomials.allocator
	if e = a.WriteUint32(v.raw, o, uint32(x.owner)); e != nil {
		return e
	}
	if e = a.WriteUint32(v.raw, o+4, uint32(x.owner>>32)); e != nil {
		return e
	}
	if e = a.WriteUint32(v.raw, o+8, x.slot); e != nil {
		return e
	}
	return a.WriteUint32(v.raw, o+12, x.generation)
}
func (m *MultivariatePolynomialManager) lexApplyPermutation(p Polynomial, start, n uint32, numerals bool) error {
	c := m.core
	perm := m.lexPermutation
	as, e := c.captureCoefficientPointer(p, start)
	if e != nil {
		return e
	}
	ms, e := c.captureMonomialPointer(p, start)
	if e != nil {
		return e
	}
	for i := int32(0); i < int32(n); i++ {
		if uint64(i) >= uint64(len(perm)) {
			return arena.ErrBounds
		}
		x := perm[i]
		if int32(x) < 0 {
			continue
		}
		j := i
		for {
			if polynomialSourceDebug && j < 0 {
				return ErrPolynomialAssertion
			}
			if uint64(j) >= uint64(len(perm)) {
				return arena.ErrBounds
			}
			next := perm[j]
			k := int32(next)
			if polynomialSourceDebug && (k < 0 || k >= int32(n)) {
				return ErrPolynomialAssertion
			}
			if k == math.MinInt32 {
				return arena.ErrBounds
			}
			perm[j] = uint32(-k - 1)
			if k == i {
				break
			}
			if numerals {
				a, e := as.read(uint32(j))
				if e != nil {
					return e
				}
				b, e := as.read(uint32(k))
				if e != nil {
					return e
				}
				// apply_permutation uses representation swap, not numeral_manager.swap.
				a, b = b, a
				if e = as.write(uint32(j), a); e != nil {
					return e
				}
				if e = as.write(uint32(k), b); e != nil {
					return e
				}
			} else {
				a, e := ms.read(uint32(j))
				if e != nil {
					return e
				}
				b, e := ms.read(uint32(k))
				if e != nil {
					return e
				}
				if e = ms.write(uint32(j), b); e != nil {
					return e
				}
				if e = ms.write(uint32(k), a); e != nil {
					return e
				}
			}
			j = k
		}
	}
	if numerals {
		for i := uint32(0); i < n; i++ {
			if uint64(i) >= uint64(len(perm)) {
				return arena.ErrBounds
			}
			x := perm[i]
			if int32(x) == math.MinInt32 {
				return arena.ErrBounds
			}
			perm[i] = uint32(-int32(x) - 1)
		}
	}
	return nil
}
