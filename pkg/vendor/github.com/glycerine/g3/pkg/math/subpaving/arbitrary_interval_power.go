package subpaving

// Source: src/math/interval/interval_def.h:1271-1378, concrete config_mpf.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Power follows the source's positive, negative, crossing-zero and odd cases.
// Scratch bounds preserve the source when the destination aliases it.
func (v *ArbitraryInterval) Power(src *ArbitraryInterval, n uint32) (err error) {
	if err = v.pair(src); err != nil {
		return err
	}
	if n == 1 {
		return v.SetFrom(src)
	}
	m := v.numbers
	var lowerValue, upperValue numeral.ArbitraryFloat
	lo, hi := &lowerValue, &upperValue
	defer func() { err = errors.Join(err, m.Del(lo), m.Del(hi)) }()
	srcLower, srcUpper, err := src.endpoints()
	if err != nil {
		return err
	}
	lowerInf, upperInf := src.lowerInf, src.upperInf
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	if n%2 != 0 {
		// The source's odd branch uses the manager's ambient rounding.
		if !lowerInf {
			if err = m.Power(srcLower, n, lo); err != nil {
				return err
			}
		}
		if !upperInf {
			if err = m.Power(srcUpper, n, hi); err != nil {
				return err
			}
		}
	} else {
		positive, e := (interval.ArbitraryExtended{Numbers: m}).IsPos(srcLower, src.lowerKind())
		if e != nil {
			return e
		}
		negative, e := (interval.ArbitraryExtended{Numbers: m}).IsNeg(srcUpper, src.upperKind())
		if e != nil {
			return e
		}
		switch {
		case positive:
			m.RoundToMinusInf()
			if err = m.Power(srcLower, n, lo); err != nil {
				return err
			}
			if !upperInf {
				m.RoundToPlusInf()
				if err = m.Power(srcUpper, n, hi); err != nil {
					return err
				}
			}
		case negative:
			m.RoundToMinusInf()
			if err = m.Power(srcUpper, n, lo); err != nil {
				return err
			}
			if !lowerInf {
				m.RoundToPlusInf()
				if err = m.Power(srcLower, n, hi); err != nil {
					return err
				}
			}
			lowerInf, upperInf = false, lowerInf
			lowerOpen, upperOpen = upperOpen, lowerOpen
		default:
			var leftValue, rightValue numeral.ArbitraryFloat
			left, right := &leftValue, &rightValue
			defer func() { err = errors.Join(err, m.Del(left), m.Del(right)) }()
			if !lowerInf {
				if err = m.Copy(left, srcLower); err != nil {
					return err
				}
			}
			if !upperInf {
				if err = m.Copy(right, srcUpper); err != nil {
					return err
				}
			}
			m.RoundToPlusInf()
			extended := interval.ArbitraryExtended{Numbers: m}
			lk, e := extended.Power(left, src.lowerKind(), n)
			if e != nil {
				return e
			}
			rk, e := extended.Power(right, src.upperKind(), n)
			if e != nil {
				return e
			}
			pickLeft, e := extended.Greater(left, lk, right, rk)
			if e != nil {
				return e
			}
			if !pickLeft {
				equal, e1 := extended.Equal(left, lk, right, rk)
				if e1 != nil {
					return e1
				}
				pickLeft = equal && !lowerOpen && upperOpen
			}
			if pickLeft {
				if err = m.Swap(hi, left); err != nil {
					return err
				}
				upperInf, upperOpen = lk == interval.PlusInfinity, lowerOpen
			} else {
				if err = m.Swap(hi, right); err != nil {
					return err
				}
				upperInf = rk == interval.PlusInfinity
			}
			if err = m.Reset(lo); err != nil {
				return err
			}
			lowerInf, lowerOpen = false, false
		}
	}
	dstLower, dstUpper, err := v.endpoints()
	if err != nil {
		return err
	}
	if err = m.Swap(dstLower, lo); err != nil {
		return err
	}
	if err = m.Swap(dstUpper, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lowerInf, upperInf
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
