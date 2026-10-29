package interval

// Source: src/math/interval/interval_def.h:1672-1849, concrete mpq interval.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Div follows the source interval quotient branches. The numerator cannot be
// the destination, matching the source precondition; denominator aliasing is
// safe because endpoint products publish from owned scratch numerals.
func (v *RationalInterval) Div(num, den *RationalInterval) (err error) {
	if err = v.binaryOperands(num, den); err != nil {
		return err
	}
	if v == num {
		return arena.ErrInvalidSlot
	}
	contains, err := den.ContainsZero()
	if err != nil {
		return err
	}
	if contains {
		return ErrExtendedDomain
	}
	zero, err := num.isSingletonZero()
	if err != nil {
		return err
	}
	if zero {
		if err = v.numbers.Reset(&v.lower); err != nil {
			return err
		}
		if err = v.numbers.Reset(&v.upper); err != nil {
			return err
		}
		v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen = false, false, false, false
		return nil
	}
	nc, err := num.signClass()
	if err != nil {
		return err
	}
	e := RationalExtended{Numbers: v.numbers}
	negative, err := e.IsNeg(&den.upper, den.upperKind())
	if err != nil {
		return err
	}
	upperZero, err := e.IsZero(&den.upper, den.upperKind())
	if err != nil {
		return err
	}
	lowerPositive, err := e.IsPos(&den.lower, den.lowerKind())
	if err != nil {
		return err
	}
	lowerZero, err := e.IsZero(&den.lower, den.lowerKind())
	if err != nil {
		return err
	}
	dn := negative || upperZero && den.upperOpen
	dp := lowerPositive || lowerZero && den.lowerOpen
	if !dn && !dp {
		return ErrExtendedDomain
	}
	a, b := num.endpoints()
	c, d := den.endpoints()
	lower, upper := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, v.numbers.Del(&lower), v.numbers.Del(&upper)) }()
	var le, ld, ue, ud intervalEndpoint
	var lo, uo bool
	var lk, uk ExtendedKind
	infLower, infUpper := false, false
	if dn {
		switch nc {
		case 'N':
			le, ld, ue, ud = b, c, a, d
			n0, x := num.closedZeroAtUpper()
			if x != nil {
				return x
			}
			lo = !n0 && (b.open || c.open)
			uo = a.open || d.open
			infUpper = upperZero
		case 'M':
			le, ld, ue, ud = b, d, a, d
			lo = b.open || d.open
			uo = a.open || d.open
			infLower, infUpper = upperZero, upperZero
		default: // P / N1
			le, ld, ue, ud = b, d, a, c
			p0, x := num.closedZeroAtLower()
			if x != nil {
				return x
			}
			lo = b.open || d.open
			uo = !p0 && (a.open || c.open)
			infLower = upperZero
		}
	} else {
		switch nc {
		case 'N':
			le, ld, ue, ud = a, c, b, d
			n0, x := num.closedZeroAtUpper()
			if x != nil {
				return x
			}
			lo = a.open || c.open
			uo = !n0 && (b.open || d.open)
			infLower = lowerZero
		case 'M':
			le, ld, ue, ud = a, c, b, c
			lo = a.open || c.open
			uo = b.open || c.open
			infLower, infUpper = lowerZero, lowerZero
		default: // P / P1
			le, ld, ue, ud = a, d, b, c
			p0, x := num.closedZeroAtLower()
			if x != nil {
				return x
			}
			lo = !p0 && (a.open || d.open)
			uo = b.open || c.open
			infUpper = lowerZero
		}
	}
	if infLower {
		lk = MinusInfinity
	} else {
		lk, err = e.Div(le.value, le.kind, ld.value, ld.kind, &lower)
		if err != nil {
			return err
		}
	}
	if infUpper {
		uk = PlusInfinity
	} else {
		uk, err = e.Div(ue.value, ue.kind, ud.value, ud.kind, &upper)
		if err != nil {
			return err
		}
	}
	if err = v.numbers.Swap(&v.lower, &lower); err != nil {
		return err
	}
	if err = v.numbers.Swap(&v.upper, &upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == MinusInfinity, uk == PlusInfinity
	v.lowerOpen, v.upperOpen = lo, uo
	return v.CheckInvariant()
}
