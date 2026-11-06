package subpaving

// Source: src/math/interval/interval_def.h:1672-1849, concrete config_mpff.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
)

// Div follows the source's six numerator-sign and denominator-sign branches.
// The numerator cannot alias the result, as required by the source routine.
func (v *FastInterval) Div(num, den *FastInterval) (err error) {
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
		dstLower, dstUpper, e := v.endpoints()
		if e != nil {
			return e
		}
		if err = v.numbers.Reset(dstLower); err != nil {
			return err
		}
		if err = v.numbers.Reset(dstUpper); err != nil {
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
	e := interval.FastExtended{Numbers: m}
	denLower, denUpper, err := den.endpoints()
	if err != nil {
		return err
	}
	negative, err := e.IsNeg(denUpper, den.upperKind())
	if err != nil {
		return err
	}
	upperZero, err := e.IsZero(denUpper, den.upperKind())
	if err != nil {
		return err
	}
	positive, err := e.IsPos(denLower, den.lowerKind())
	if err != nil {
		return err
	}
	lowerZero, err := e.IsZero(denLower, den.lowerKind())
	if err != nil {
		return err
	}
	dn := negative || upperZero && den.upperOpen
	dp := positive || lowerZero && den.lowerOpen
	if !dn && !dp {
		return interval.ErrExtendedDomain
	}
	a, b, err := num.boundEndpoints()
	if err != nil {
		return err
	}
	c, d, err := den.boundEndpoints()
	if err != nil {
		return err
	}
	lowerOwner, upperOwner, err := fastScratchPair(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lowerOwner.Close(), upperOwner.Close()) }()
	lower, err := lowerOwner.Get()
	if err != nil {
		return err
	}
	upper, err := upperOwner.Get()
	if err != nil {
		return err
	}
	var le, ld, ue, ud fastEndpoint
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
	dstLower, dstUpper, err := v.endpoints()
	if err != nil {
		return err
	}
	if err = m.Swap(dstLower, lower); err != nil {
		return err
	}
	if err = m.Swap(dstUpper, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
