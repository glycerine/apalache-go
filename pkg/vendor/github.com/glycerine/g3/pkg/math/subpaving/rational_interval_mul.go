package subpaving

// Source: src/math/interval/interval_def.h:1019-1218, concrete config_mpq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type intervalEndpoint struct {
	value *numeral.Rational
	kind  interval.ExtendedKind
	open  bool
}

func (v *RationalInterval) endpoints() (intervalEndpoint, intervalEndpoint, error) {
	lo, hi, err := v.resolved()
	if err != nil {
		return intervalEndpoint{}, intervalEndpoint{}, err
	}
	return intervalEndpoint{lo, v.lowerKind(), v.lowerOpen}, intervalEndpoint{hi, v.upperKind(), v.upperOpen}, nil
}

func (v *RationalInterval) signClass() (byte, error) {
	lo, hi, err := v.resolved()
	if err != nil {
		return 0, err
	}
	e := interval.RationalExtended{Numbers: v.numbers}
	neg, err := e.IsNeg(hi, v.upperKind())
	if err != nil {
		return 0, err
	}
	z, err := e.IsZero(hi, v.upperKind())
	if err != nil {
		return 0, err
	}
	if neg || z {
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

func (v *RationalInterval) closedZeroAtUpper() (bool, error) {
	_, hi, err := v.resolved()
	if err != nil {
		return false, err
	}
	if v.upperInf {
		return false, nil
	}
	z, err := v.numbers.IsZero(hi)
	if err != nil {
		return false, err
	}
	return z && !v.upperOpen, nil
}

func (v *RationalInterval) closedZeroAtLower() (bool, error) {
	lo, _, err := v.resolved()
	if err != nil {
		return false, err
	}
	if v.lowerInf {
		return false, nil
	}
	z, err := v.numbers.IsZero(lo)
	if err != nil {
		return false, err
	}
	return z && !v.lowerOpen, nil
}

func (v *RationalInterval) isSingletonZero() (bool, error) {
	lo, hi, err := v.resolved()
	if err != nil {
		return false, err
	}
	if v.lowerInf || v.upperInf {
		return false, nil
	}
	l, err := v.numbers.IsZero(lo)
	if err != nil {
		return false, err
	}
	u, err := v.numbers.IsZero(hi)
	if err != nil {
		return false, err
	}
	return l && u, nil
}

// Mul computes the source's nine sign-class branches, including the mixed x
// mixed tie rule that prefers a closed endpoint over an open one.
func (v *RationalInterval) Mul(a, b *RationalInterval) (err error) {
	if _, _, err = v.pairView(a); err != nil {
		return err
	}
	if _, _, err = v.pairView(b); err != nil {
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
	al, au, err := a.endpoints()
	if err != nil {
		return err
	}
	bl, bu, err := b.endpoints()
	if err != nil {
		return err
	}
	lower, upper := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, v.numbers.Del(&lower), v.numbers.Del(&upper)) }()
	e := interval.RationalExtended{Numbers: v.numbers}
	var le, re, ue, ve intervalEndpoint
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
		lo = al.open || bu.open
		uo = al.open || bl.open
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
		lo = al.open || bu.open
		uo = !(az || bz) && (au.open || bl.open)
	case ac == 'M' && bc == 'N':
		le, re, ue, ve = au, bl, al, bl
		lo = au.open || bl.open
		uo = al.open || bl.open
	case ac == 'M' && bc == 'P':
		le, re, ue, ve = al, bu, au, bu
		lo = al.open || bu.open
		uo = au.open || bu.open
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
		lo = au.open || bl.open
		uo = !(az || bz) && (al.open || bu.open)
	case ac == 'P' && bc == 'M':
		le, re, ue, ve = au, bl, au, bu
		lo = au.open || bl.open
		uo = au.open || bu.open
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
		lo = !(az || bz) && (al.open || bl.open)
		uo = au.open || bu.open
	default: // mixed x mixed: select extrema of four exact products.
		var ad, bcval, acval, bd numeral.Rational
		ad, bcval, acval, bd = numeral.NewRational(), numeral.NewRational(), numeral.NewRational(), numeral.NewRational()
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
			le = intervalEndpoint{&ad, adk, adOpen}
		} else {
			le = intervalEndpoint{&bcval, bck, bcOpen}
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
			ue = intervalEndpoint{&acval, ack, acOpen}
		} else {
			ue = intervalEndpoint{&bd, bdk, bdOpen}
		}
		lo, uo = le.open, ue.open
		if err = v.numbers.Swap(&lower, le.value); err != nil {
			return err
		}
		if err = v.numbers.Swap(&upper, ue.value); err != nil {
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
