package numeral

// Source: src/util/mpq.h/.cpp number-theory overloads.

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (m *RationalManager) GCD(a, b, out *Rational) error {
	if e := m.integerOperandsAt(a, b, 602); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if e := m.Z.GCD(&a.Num, &b.Num, &out.Num); e != nil {
		return e
	}
	return m.resetDenominator(out)
}

// Unlike pair gcd, the source zero/singleton branches do not require integers.
func (m *RationalManager) GCDMany(as []Rational, g *Rational) error {
	if uint64(len(as)) > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	if e := m.check(g); e != nil {
		return e
	}
	switch len(as) {
	case 0:
		return m.Reset(g)
	case 1:
		if e := m.Copy(g, &as[0]); e != nil {
			return e
		}
		return m.Abs(g)
	}
	if e := m.GCD(&as[0], &as[1], g); e != nil {
		return e
	}
	for i := 2; i < len(as); i++ {
		if isIntegerRational(g) && isOneInteger(&g.Num) {
			return nil
		}
		if e := m.GCD(g, &as[i], g); e != nil {
			return e
		}
	}
	return nil
}

// Source resets all three output denominators BEFORE the integer algorithm.
// Their published prefixes remain even if a later local allocation fails.
func (m *RationalManager) ExtendedGCD(r1, r2, a, b, g *Rational) error {
	return m.extendedGCDWithAllocationContext(r1, r2, a, b, g, integerAllocationInvocationContext{})
}
func (m *RationalManager) extendedGCDWithAllocationContext(r1, r2, a, b, g *Rational, context integerAllocationInvocationContext) error {
	if e := m.integerOperandsAt(r1, r2, 610); e != nil {
		return e
	}
	for _, q := range []*Rational{a, b, g} {
		if e := m.check(q); e != nil {
			return e
		}
	}
	if e := m.resetDenominator(a); e != nil {
		return e
	}
	if e := m.resetDenominator(b); e != nil {
		return e
	}
	if e := m.resetDenominator(g); e != nil {
		return e
	}
	return m.Z.extendedGCDWithAllocationContext(&r1.Num, &r2.Num, &a.Num, &b.Num, &g.Num, context)
}
func (m *RationalManager) LCM(a, b, out *Rational) error {
	if e := m.integerOperandsAt(a, b, 620); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if e := m.Z.LCM(&a.Num, &b.Num, &out.Num); e != nil {
		return e
	}
	return m.resetDenominator(out)
}
func (m *RationalManager) Divides(a, b *Rational) (bool, error) {
	if e := m.integerOperandsAt(a, b, 628); e != nil {
		return false, e
	}
	return m.Z.Divides(&a.Num, &b.Num)
}
