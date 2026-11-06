package subpaving

// Source: src/math/interval/interval_def.h:1672-1849, concrete config_mpfx.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
)

// Div follows the source's six numerator-sign and denominator-sign branches.
// The numerator cannot alias the result, as required by the source routine.
func (v *FixedInterval) Div(num, den *FixedInterval) (err error) {
	if err = v.pair(num); err != nil {
		return err
	}
	if err = v.pair(den); err != nil {
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
	zero, err := num.isSingletonZero()
	if err != nil {
		return err
	}
	if zero {
		if err = v.numbers.Reset(v.lower); err != nil {
			return err
		}
		if err = v.numbers.Reset(v.upper); err != nil {
			return err
		}
		v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen = false, false, false, false
		return nil
	}
	nc, err := num.signClass()
	if err != nil {
		return err
	}
	m := v.numbers
	e := interval.FixedExtended{Numbers: m}
	negative, err := e.IsNeg(den.upper, den.upperKind())
	if err != nil {
		return err
	}
	upperZero, err := e.IsZero(den.upper, den.upperKind())
	if err != nil {
		return err
	}
	positive, err := e.IsPos(den.lower, den.lowerKind())
	if err != nil {
		return err
	}
	lowerZero, err := e.IsZero(den.lower, den.lowerKind())
	if err != nil {
		return err
	}
	dn := negative || upperZero && den.upperOpen
	dp := positive || lowerZero && den.lowerOpen
	if !dn && !dp {
		return interval.ErrExtendedDomain
	}
	a, b := num.endpoints()
	c, d := den.endpoints()
	lower, upper, err := fixedScratchPair(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lower.Close(), upper.Close()) }()
	var le, ld, ue, ud fixedEndpoint
	var lowerOpen, upperOpen bool
	var lk, uk interval.ExtendedKind
	infLower, infUpper := false, false
	if dn {
		switch nc {
		case 'N':
			le, ld, ue, ud = b, c, a, d
			n0, x := num.closedZeroAtUpper()
			if x != nil {
				return x
			}
			lowerOpen = !n0 && (b.open || c.open)
			upperOpen = a.open || d.open
			infUpper = upperZero
		case 'M':
			le, ld, ue, ud = b, d, a, d
			lowerOpen, upperOpen = b.open || d.open, a.open || d.open
			infLower, infUpper = upperZero, upperZero
		default: // P / N1
			le, ld, ue, ud = b, d, a, c
			p0, x := num.closedZeroAtLower()
			if x != nil {
				return x
			}
			lowerOpen = b.open || d.open
			upperOpen = !p0 && (a.open || c.open)
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
			lowerOpen = a.open || c.open
			upperOpen = !n0 && (b.open || d.open)
			infLower = lowerZero
		case 'M':
			le, ld, ue, ud = a, c, b, c
			lowerOpen, upperOpen = a.open || c.open, b.open || c.open
			infLower, infUpper = lowerZero, lowerZero
		default: // P / P1
			le, ld, ue, ud = a, d, b, c
			p0, x := num.closedZeroAtLower()
			if x != nil {
				return x
			}
			lowerOpen = !p0 && (a.open || d.open)
			upperOpen = b.open || c.open
			infUpper = lowerZero
		}
	}
	if infLower {
		lk = interval.MinusInfinity
	} else {
		if err = m.SetRounding(false); err != nil {
			return err
		}
		lk, err = e.Div(le.value, le.kind, ld.value, ld.kind, lower)
		if err != nil {
			return err
		}
	}
	if infUpper {
		uk = interval.PlusInfinity
	} else {
		if err = m.SetRounding(true); err != nil {
			return err
		}
		uk, err = e.Div(ue.value, ue.kind, ud.value, ud.kind, upper)
		if err != nil {
			return err
		}
	}
	if err = m.Swap(v.lower, lower); err != nil {
		return err
	}
	if err = m.Swap(v.upper, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
