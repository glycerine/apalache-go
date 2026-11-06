package subpaving

// Source: src/math/interval/interval_def.h:1017-1218, concrete config_hwf.

import (
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type hardwareEndpoint struct {
	value *numeral.HardwareFloat
	kind  interval.ExtendedKind
	open  bool
}

func (v *HardwareInterval) boundEndpoints() (hardwareEndpoint, hardwareEndpoint) {
	lo, hi, _ := v.endpoints()
	return hardwareEndpoint{lo, v.lowerKind(), v.lowerOpen}, hardwareEndpoint{hi, v.upperKind(), v.upperOpen}
}
func (v *HardwareInterval) signClass() byte {
	lo, hi, _ := v.endpoints()
	e := interval.HardwareExtended{Numbers: v.numbers}
	if e.IsNeg(hi, v.upperKind()) || e.IsZero(hi, v.upperKind()) {
		return 'N'
	}
	if e.IsNeg(lo, v.lowerKind()) {
		return 'M'
	}
	return 'P'
}
func (v *HardwareInterval) closedZeroAtUpper() bool {
	_, hi, _ := v.endpoints()
	return !v.upperInf && v.numbers.IsZero(hi) && !v.upperOpen
}
func (v *HardwareInterval) closedZeroAtLower() bool {
	lo, _, _ := v.endpoints()
	return !v.lowerInf && v.numbers.IsZero(lo) && !v.lowerOpen
}
func (v *HardwareInterval) isSingletonZero() bool {
	if v.lowerInf || v.upperInf {
		return false
	}
	lo, hi, _ := v.endpoints()
	return v.numbers.IsZero(lo) && v.numbers.IsZero(hi)
}

// Mul follows all nine source sign classes, including mixed extrema ties.
// Local scalar results leave aliased inputs intact until publication.
func (v *HardwareInterval) Mul(a, b *HardwareInterval) error {
	if err := v.writablePair(a); err != nil {
		return err
	}
	if err := v.pair(b); err != nil {
		return err
	}
	if a.isSingletonZero() {
		return v.SetFrom(a)
	}
	if b.isSingletonZero() {
		return v.SetFrom(b)
	}
	ac, bc := a.signClass(), b.signClass()
	al, au := a.boundEndpoints()
	bl, bu := b.boundEndpoints()
	m := v.numbers
	e := interval.HardwareExtended{Numbers: m}
	var lower, upper numeral.HardwareFloat
	var le, re, ue, ve hardwareEndpoint
	var lowerOpen, upperOpen bool
	var lk, uk interval.ExtendedKind
	switch {
	case ac == 'N' && bc == 'N':
		le, re, ue, ve = au, bu, al, bl
		lowerOpen = !(a.closedZeroAtUpper() || b.closedZeroAtUpper()) && (au.open || bu.open)
		upperOpen = al.open || bl.open
	case ac == 'N' && bc == 'M':
		le, re, ue, ve = al, bu, al, bl
		lowerOpen, upperOpen = al.open || bu.open, al.open || bl.open
	case ac == 'N' && bc == 'P':
		le, re, ue, ve = al, bu, au, bl
		lowerOpen = al.open || bu.open
		upperOpen = !(a.closedZeroAtUpper() || b.closedZeroAtLower()) && (au.open || bl.open)
	case ac == 'M' && bc == 'N':
		le, re, ue, ve = au, bl, al, bl
		lowerOpen, upperOpen = au.open || bl.open, al.open || bl.open
	case ac == 'M' && bc == 'P':
		le, re, ue, ve = al, bu, au, bu
		lowerOpen, upperOpen = al.open || bu.open, au.open || bu.open
	case ac == 'P' && bc == 'N':
		le, re, ue, ve = au, bl, al, bu
		lowerOpen = au.open || bl.open
		upperOpen = !(a.closedZeroAtLower() || b.closedZeroAtUpper()) && (al.open || bu.open)
	case ac == 'P' && bc == 'M':
		le, re, ue, ve = au, bl, au, bu
		lowerOpen, upperOpen = au.open || bl.open, au.open || bu.open
	case ac == 'P' && bc == 'P':
		le, re, ue, ve = al, bl, au, bu
		lowerOpen = !(a.closedZeroAtLower() || b.closedZeroAtLower()) && (al.open || bl.open)
		upperOpen = au.open || bu.open
	default:
		var ad, bcval, acval, bd numeral.HardwareFloat
		m.RoundToMinusInf()
		adk, err := e.Mul(al.value, al.kind, bu.value, bu.kind, &ad)
		if err != nil {
			return err
		}
		bck, err := e.Mul(au.value, au.kind, bl.value, bl.kind, &bcval)
		if err != nil {
			return err
		}
		m.RoundToPlusInf()
		ack, err := e.Mul(al.value, al.kind, bl.value, bl.kind, &acval)
		if err != nil {
			return err
		}
		bdk, err := e.Mul(au.value, au.kind, bu.value, bu.kind, &bd)
		if err != nil {
			return err
		}
		adOpen, bcOpen := al.open || bu.open, au.open || bl.open
		acOpen, bdOpen := al.open || bl.open, au.open || bu.open
		pickAD := e.Less(&ad, adk, &bcval, bck)
		if !pickAD {
			pickAD = e.Equal(&ad, adk, &bcval, bck) && !adOpen && bcOpen
		}
		if pickAD {
			lower = ad
			lk, lowerOpen = adk, adOpen
		} else {
			lower = bcval
			lk, lowerOpen = bck, bcOpen
		}
		pickAC := e.Greater(&acval, ack, &bd, bdk)
		if !pickAC {
			pickAC = e.Equal(&acval, ack, &bd, bdk) && !acOpen && bdOpen
		}
		if pickAC {
			upper = acval
			uk, upperOpen = ack, acOpen
		} else {
			upper = bd
			uk, upperOpen = bdk, bdOpen
		}
	}
	if ac != 'M' || bc != 'M' {
		m.RoundToMinusInf()
		var err error
		lk, err = e.Mul(le.value, le.kind, re.value, re.kind, &lower)
		if err != nil {
			return err
		}
		m.RoundToPlusInf()
		uk, err = e.Mul(ue.value, ue.kind, ve.value, ve.kind, &upper)
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
