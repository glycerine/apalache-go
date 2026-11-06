package subpaving

// Source: src/math/interval/interval_def.h:1663-1849, concrete config_hwf.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Div follows the six numerator and denominator sign combinations. As in the
// source routine, the numerator cannot alias the result; the denominator may.
func (v *HardwareInterval) Div(num, den *HardwareInterval) error {
	if err := v.writablePair(num); err != nil {
		return err
	}
	if err := v.pair(den); err != nil {
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
		return interval.ErrExtendedDomain
	}
	if num.isSingletonZero() {
		lo, hi, _ := v.endpoints()
		if err := v.numbers.Reset(lo); err != nil {
			return err
		}
		if err := v.numbers.Reset(hi); err != nil {
			return err
		}
		v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen = false, false, false, false
		return nil
	}
	nc := num.signClass()
	c, d := den.boundEndpoints()
	e := interval.HardwareExtended{Numbers: v.numbers}
	negative := e.IsNeg(d.value, d.kind)
	upperZero := e.IsZero(d.value, d.kind)
	positive := e.IsPos(c.value, c.kind)
	lowerZero := e.IsZero(c.value, c.kind)
	dn := negative || upperZero && d.open
	dp := positive || lowerZero && c.open
	if !dn && !dp {
		return interval.ErrExtendedDomain
	}
	a, b := num.boundEndpoints()
	var le, ld, ue, ud hardwareEndpoint
	var lowerOpen, upperOpen, infLower, infUpper bool
	if dn {
		switch nc {
		case 'N':
			le, ld, ue, ud = b, c, a, d
			lowerOpen = !num.closedZeroAtUpper() && (b.open || c.open)
			upperOpen = a.open || d.open
			infUpper = upperZero
		case 'M':
			le, ld, ue, ud = b, d, a, d
			lowerOpen, upperOpen = b.open || d.open, a.open || d.open
			infLower, infUpper = upperZero, upperZero
		default: // P / N1
			le, ld, ue, ud = b, d, a, c
			lowerOpen = b.open || d.open
			upperOpen = !num.closedZeroAtLower() && (a.open || c.open)
			infLower = upperZero
		}
	} else {
		switch nc {
		case 'N':
			le, ld, ue, ud = a, c, b, d
			lowerOpen = a.open || c.open
			upperOpen = !num.closedZeroAtUpper() && (b.open || d.open)
			infLower = lowerZero
		case 'M':
			le, ld, ue, ud = a, c, b, c
			lowerOpen, upperOpen = a.open || c.open, b.open || c.open
			infLower, infUpper = lowerZero, lowerZero
		default: // P / P1
			le, ld, ue, ud = a, d, b, c
			lowerOpen = !num.closedZeroAtLower() && (a.open || d.open)
			upperOpen = b.open || c.open
			infUpper = lowerZero
		}
	}
	m := v.numbers
	var lower, upper numeral.HardwareFloat
	lk, uk := interval.MinusInfinity, interval.PlusInfinity
	if !infLower {
		m.RoundToMinusInf()
		lk, err = e.Div(le.value, le.kind, ld.value, ld.kind, &lower)
		if err != nil {
			return err
		}
	}
	if !infUpper {
		m.RoundToPlusInf()
		uk, err = e.Div(ue.value, ue.kind, ud.value, ud.kind, &upper)
		if err != nil {
			return err
		}
	}
	dstLower, dstUpper, _ := v.endpoints()
	*dstLower, *dstUpper = lower, upper
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
