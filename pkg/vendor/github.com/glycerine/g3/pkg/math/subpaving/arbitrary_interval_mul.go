package subpaving

// Source: src/math/interval/interval_def.h:1017-1214, concrete config_mpf.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type arbitraryEndpoint struct {
	value *numeral.ArbitraryFloat
	kind  interval.ExtendedKind
	open  bool
}

func (v *ArbitraryInterval) boundEndpoints() (arbitraryEndpoint, arbitraryEndpoint, error) {
	lo, hi, err := v.endpoints()
	return arbitraryEndpoint{lo, v.lowerKind(), v.lowerOpen}, arbitraryEndpoint{hi, v.upperKind(), v.upperOpen}, err
}

func (v *ArbitraryInterval) signClass() (byte, error) {
	lo, hi, err := v.endpoints()
	if err != nil {
		return 0, err
	}
	e := interval.ArbitraryExtended{Numbers: v.numbers}
	neg, err := e.IsNeg(hi, v.upperKind())
	if err != nil {
		return 0, err
	}
	zero, err := e.IsZero(hi, v.upperKind())
	if err != nil {
		return 0, err
	}
	if neg || zero {
		return 'N', nil
	}
	neg, err = e.IsNeg(lo, v.lowerKind())
	if err != nil {
		return 0, err
	}
	if neg {
		return 'M', nil
	}
	return 'P', nil
}

func (v *ArbitraryInterval) closedZeroAtUpper() (bool, error) {
	_, hi, err := v.endpoints()
	if err != nil {
		return false, err
	}
	if v.upperInf {
		return false, nil
	}
	zero, err := v.numbers.IsZero(hi)
	return zero && !v.upperOpen, err
}

func (v *ArbitraryInterval) closedZeroAtLower() (bool, error) {
	lo, _, err := v.endpoints()
	if err != nil {
		return false, err
	}
	if v.lowerInf {
		return false, nil
	}
	zero, err := v.numbers.IsZero(lo)
	return zero && !v.lowerOpen, err
}

func (v *ArbitraryInterval) isSingletonZero() (bool, error) {
	lo, hi, err := v.endpoints()
	if err != nil {
		return false, err
	}
	if v.lowerInf || v.upperInf {
		return false, nil
	}
	lowerZero, err := v.numbers.IsZero(lo)
	if err != nil {
		return false, err
	}
	upperZero, err := v.numbers.IsZero(hi)
	return lowerZero && upperZero, err
}

// Mul follows all nine source sign-class branches with directed rounding.
// Local owned bounds keep either input intact when it aliases the destination.
func (v *ArbitraryInterval) Mul(a, b *ArbitraryInterval) (err error) {
	if err = v.pair(a); err != nil {
		return err
	}
	if err = v.pair(b); err != nil {
		return err
	}
	zero, err := a.isSingletonZero()
	if err != nil {
		return err
	}
	if zero {
		return v.SetFrom(a)
	}
	zero, err = b.isSingletonZero()
	if err != nil {
		return err
	}
	if zero {
		return v.SetFrom(b)
	}
	ac, err := a.signClass()
	if err != nil {
		return err
	}
	bc, err := b.signClass()
	if err != nil {
		return err
	}
	al, au, err := a.boundEndpoints()
	if err != nil {
		return err
	}
	bl, bu, err := b.boundEndpoints()
	if err != nil {
		return err
	}
	m := v.numbers
	var lowerValue, upperValue numeral.ArbitraryFloat
	lower, upper := &lowerValue, &upperValue
	defer func() { err = errors.Join(err, m.Del(lower), m.Del(upper)) }()
	e := interval.ArbitraryExtended{Numbers: m}
	var le, re, ue, ve arbitraryEndpoint
	var lowerOpen, upperOpen bool
	var lk, uk interval.ExtendedKind
	switch {
	case ac == 'N' && bc == 'N':
		le, re, ue, ve = au, bu, al, bl
		az, x := a.closedZeroAtUpper()
		if x != nil {
			return x
		}
		bz, x := b.closedZeroAtUpper()
		if x != nil {
			return x
		}
		lowerOpen = !(az || bz) && (au.open || bu.open)
		upperOpen = al.open || bl.open
	case ac == 'N' && bc == 'M':
		le, re, ue, ve = al, bu, al, bl
		lowerOpen, upperOpen = al.open || bu.open, al.open || bl.open
	case ac == 'N' && bc == 'P':
		le, re, ue, ve = al, bu, au, bl
		az, x := a.closedZeroAtUpper()
		if x != nil {
			return x
		}
		bz, x := b.closedZeroAtLower()
		if x != nil {
			return x
		}
		lowerOpen = al.open || bu.open
		upperOpen = !(az || bz) && (au.open || bl.open)
	case ac == 'M' && bc == 'N':
		le, re, ue, ve = au, bl, al, bl
		lowerOpen, upperOpen = au.open || bl.open, al.open || bl.open
	case ac == 'M' && bc == 'P':
		le, re, ue, ve = al, bu, au, bu
		lowerOpen, upperOpen = al.open || bu.open, au.open || bu.open
	case ac == 'P' && bc == 'N':
		le, re, ue, ve = au, bl, al, bu
		az, x := a.closedZeroAtLower()
		if x != nil {
			return x
		}
		bz, x := b.closedZeroAtUpper()
		if x != nil {
			return x
		}
		lowerOpen = au.open || bl.open
		upperOpen = !(az || bz) && (al.open || bu.open)
	case ac == 'P' && bc == 'M':
		le, re, ue, ve = au, bl, au, bu
		lowerOpen, upperOpen = au.open || bl.open, au.open || bu.open
	case ac == 'P' && bc == 'P':
		le, re, ue, ve = al, bl, au, bu
		az, x := a.closedZeroAtLower()
		if x != nil {
			return x
		}
		bz, x := b.closedZeroAtLower()
		if x != nil {
			return x
		}
		lowerOpen = !(az || bz) && (al.open || bl.open)
		upperOpen = au.open || bu.open
	default: // mixed times mixed: compare the four directed products.
		var adValue, bcValue, acValue, bdValue numeral.ArbitraryFloat
		ad, bcval, acval, bd := &adValue, &bcValue, &acValue, &bdValue
		defer func() { err = errors.Join(err, m.Del(ad), m.Del(bcval), m.Del(acval), m.Del(bd)) }()
		m.RoundToMinusInf()
		adk, x := e.Mul(al.value, al.kind, bu.value, bu.kind, ad)
		if x != nil {
			return x
		}
		bck, x := e.Mul(au.value, au.kind, bl.value, bl.kind, bcval)
		if x != nil {
			return x
		}
		m.RoundToPlusInf()
		ack, x := e.Mul(al.value, al.kind, bl.value, bl.kind, acval)
		if x != nil {
			return x
		}
		bdk, x := e.Mul(au.value, au.kind, bu.value, bu.kind, bd)
		if x != nil {
			return x
		}
		adOpen, bcOpen := al.open || bu.open, au.open || bl.open
		acOpen, bdOpen := al.open || bl.open, au.open || bu.open
		pickAD, x := e.Less(ad, adk, bcval, bck)
		if x != nil {
			return x
		}
		if !pickAD {
			equal, e1 := e.Equal(ad, adk, bcval, bck)
			if e1 != nil {
				return e1
			}
			pickAD = equal && !adOpen && bcOpen
		}
		if pickAD {
			if err = m.Swap(lower, ad); err != nil {
				return err
			}
			lk, lowerOpen = adk, adOpen
		} else {
			if err = m.Swap(lower, bcval); err != nil {
				return err
			}
			lk, lowerOpen = bck, bcOpen
		}
		pickAC, x := e.Greater(acval, ack, bd, bdk)
		if x != nil {
			return x
		}
		if !pickAC {
			equal, e1 := e.Equal(acval, ack, bd, bdk)
			if e1 != nil {
				return e1
			}
			pickAC = equal && !acOpen && bdOpen
		}
		if pickAC {
			if err = m.Swap(upper, acval); err != nil {
				return err
			}
			uk, upperOpen = ack, acOpen
		} else {
			if err = m.Swap(upper, bd); err != nil {
				return err
			}
			uk, upperOpen = bdk, bdOpen
		}
	}
	if ac != 'M' || bc != 'M' {
		m.RoundToMinusInf()
		lk, err = e.Mul(le.value, le.kind, re.value, re.kind, lower)
		if err != nil {
			return err
		}
		m.RoundToPlusInf()
		uk, err = e.Mul(ue.value, ue.kind, ve.value, ve.kind, upper)
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
