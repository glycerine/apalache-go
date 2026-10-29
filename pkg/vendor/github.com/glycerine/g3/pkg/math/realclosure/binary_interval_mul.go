package realclosure

// Source: src/math/interval/interval_def.h:1019-1218, concrete mpbq interval.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type binaryIntervalEndpoint struct {
	value *numeral.BinaryRational
	kind  interval.ExtendedKind
	open  bool
}

func (v *BinaryInterval) endpoints() (binaryIntervalEndpoint, binaryIntervalEndpoint) {
	return binaryIntervalEndpoint{&v.lower, v.lowerKind(), v.lowerOpen}, binaryIntervalEndpoint{&v.upper, v.upperKind(), v.upperOpen}
}

func (v *BinaryInterval) signClass() (byte, error) {
	e := interval.BinaryExtended{Numbers: v.numbers}
	neg, err := e.IsNeg(&v.upper, v.upperKind())
	if err != nil {
		return 0, err
	}
	z, err := e.IsZero(&v.upper, v.upperKind())
	if err != nil {
		return 0, err
	}
	if neg || z {
		return 'N', nil
	}
	neg, err = e.IsNeg(&v.lower, v.lowerKind())
	if err != nil {
		return 0, err
	}
	if neg {
		return 'M', nil
	}
	return 'P', nil
}

func (v *BinaryInterval) closedZeroAtUpper() (bool, error) {
	if v.upperInf {
		return false, nil
	}
	z, err := v.numbers.IsZero(&v.upper)
	return z && !v.upperOpen, err
}

func (v *BinaryInterval) closedZeroAtLower() (bool, error) {
	if v.lowerInf {
		return false, nil
	}
	z, err := v.numbers.IsZero(&v.lower)
	return z && !v.lowerOpen, err
}

func (v *BinaryInterval) isSingletonZero() (bool, error) {
	if v.lowerInf || v.upperInf {
		return false, nil
	}
	l, err := v.numbers.IsZero(&v.lower)
	if err != nil {
		return false, err
	}
	u, err := v.numbers.IsZero(&v.upper)
	return l && u, err
}

// Mul follows the source's nine sign-class branches. Products stay in owned
// scratch numerals until publication, so the destination may alias either input.
func (v *BinaryInterval) Mul(a, b *BinaryInterval) (err error) {
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
	al, au := a.endpoints()
	bl, bu := b.endpoints()
	lower, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, v.numbers.Del(&lower), v.numbers.Del(&upper)) }()
	e := interval.BinaryExtended{Numbers: v.numbers}
	var le, re, ue, ve binaryIntervalEndpoint
	var lo, uo bool
	computed := false
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
		lo = !(az || bz) && (au.open || bu.open)
		uo = al.open || bl.open
	case ac == 'N' && bc == 'M':
		le, re, ue, ve = al, bu, al, bl
		lo, uo = al.open || bu.open, al.open || bl.open
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
		lo, uo = al.open || bu.open, !(az || bz) && (au.open || bl.open)
	case ac == 'M' && bc == 'N':
		le, re, ue, ve = au, bl, al, bl
		lo, uo = au.open || bl.open, al.open || bl.open
	case ac == 'M' && bc == 'P':
		le, re, ue, ve = al, bu, au, bu
		lo, uo = al.open || bu.open, au.open || bu.open
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
		lo, uo = au.open || bl.open, !(az || bz) && (al.open || bu.open)
	case ac == 'P' && bc == 'M':
		le, re, ue, ve = au, bl, au, bu
		lo, uo = au.open || bl.open, au.open || bu.open
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
		lo, uo = !(az || bz) && (al.open || bl.open), au.open || bu.open
	default: // mixed x mixed: choose the attained extremum on an exact tie.
		ad, bcval := numeral.NewBinaryRational(), numeral.NewBinaryRational()
		acval, bd := numeral.NewBinaryRational(), numeral.NewBinaryRational()
		defer func() {
			err = errors.Join(err, v.numbers.Del(&ad), v.numbers.Del(&bcval), v.numbers.Del(&acval), v.numbers.Del(&bd))
		}()
		adk, x := e.Mul(al.value, al.kind, bu.value, bu.kind, &ad)
		if x != nil {
			return x
		}
		bck, x := e.Mul(au.value, au.kind, bl.value, bl.kind, &bcval)
		if x != nil {
			return x
		}
		ack, x := e.Mul(al.value, al.kind, bl.value, bl.kind, &acval)
		if x != nil {
			return x
		}
		bdk, x := e.Mul(au.value, au.kind, bu.value, bu.kind, &bd)
		if x != nil {
			return x
		}
		adOpen, bcOpen := al.open || bu.open, au.open || bl.open
		acOpen, bdOpen := al.open || bl.open, au.open || bu.open
		pickAD, x := e.Less(&ad, adk, &bcval, bck)
		if x != nil {
			return x
		}
		if !pickAD {
			equal, e1 := e.Equal(&ad, adk, &bcval, bck)
			if e1 != nil {
				return e1
			}
			pickAD = equal && !adOpen && bcOpen
		}
		if pickAD {
			le = binaryIntervalEndpoint{&ad, adk, adOpen}
		} else {
			le = binaryIntervalEndpoint{&bcval, bck, bcOpen}
		}
		pickAC, x := e.Greater(&acval, ack, &bd, bdk)
		if x != nil {
			return x
		}
		if !pickAC {
			equal, e1 := e.Equal(&acval, ack, &bd, bdk)
			if e1 != nil {
				return e1
			}
			pickAC = equal && !acOpen && bdOpen
		}
		if pickAC {
			ue = binaryIntervalEndpoint{&acval, ack, acOpen}
		} else {
			ue = binaryIntervalEndpoint{&bd, bdk, bdOpen}
		}
		lo, uo = le.open, ue.open
		if err = v.numbers.Copy(&lower, le.value); err != nil {
			return err
		}
		if err = v.numbers.Copy(&upper, ue.value); err != nil {
			return err
		}
		computed = true
	}
	if !computed {
		lk, x := e.Mul(le.value, le.kind, re.value, re.kind, &lower)
		if x != nil {
			return x
		}
		uk, x := e.Mul(ue.value, ue.kind, ve.value, ve.kind, &upper)
		if x != nil {
			return x
		}
		le.kind, ue.kind = lk, uk
	}
	if err = v.numbers.Swap(&v.lower, &lower); err != nil {
		return err
	}
	if err = v.numbers.Swap(&v.upper, &upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = le.kind == interval.MinusInfinity, ue.kind == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lo, uo
	return v.CheckInvariant()
}
