package numeral

// Source: src/util/mpz.cpp/mpz.h signed comparison; explicit owner views for
// retained cross-manager algebraic interval copying.

import "github.com/glycerine/g3/pkg/arena"

// CompareFromOwners uses receiver argument scratch for small operands, as
// Compare does, and reads big operand limbs through their actual owners.
// It adds neither a numerical copy nor an allocation to source comparison.
func (m *IntegerManager) CompareFromOwners(aOwner *IntegerManager, a *Int, bOwner *IntegerManager, b *Int) (int, error) {
	if e := m.checkReady(); e != nil {
		return 0, e
	}
	if aOwner == nil || bOwner == nil {
		return 0, arena.ErrConfig
	}
	if e := aOwner.check(a); e != nil {
		return 0, e
	}
	if e := bOwner.check(b); e != nil {
		return 0, e
	}
	if a.IsSmall() && b.IsSmall() {
		if a.Val < b.Val {
			return -1, nil
		}
		if a.Val > b.Val {
			return 1, nil
		}
		return 0, nil
	}
	sa, ca, e := m.signCell(a, 0)
	if e != nil {
		return 0, e
	}
	sb, cb, e := m.signCell(b, 1)
	if e != nil {
		return 0, e
	}
	if a.IsSmall() {
		aOwner = m
	}
	if b.IsSmall() {
		bOwner = m
	}
	if sa > 0 && sb <= 0 {
		return 1, nil
	}
	if sa <= 0 && sb > 0 {
		return -1, nil
	}
	for i := max(aOwner.Cell(ca).Size, bOwner.Cell(cb).Size); i > 0; i-- {
		x, y := aOwner.Digit(ca, i-1), bOwner.Digit(cb, i-1)
		if x == y {
			continue
		}
		order := 1
		if x < y {
			order = -1
		}
		if sa <= 0 {
			order = -order
		}
		return order, nil
	}
	return 0, nil
}
