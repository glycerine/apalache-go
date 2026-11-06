package subpaving

// Source: src/math/interval/interval_def.h:1271-1378, concrete config_mpfx.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
)

// Power follows the source's positive, negative, crossing-zero and odd cases.
// Scratch bounds preserve the source when the destination aliases it.
func (v *FixedInterval) Power(src *FixedInterval, n uint32) (err error) {
	if err = v.pair(src); err != nil {
		return err
	}
	if n == 1 {
		return v.SetFrom(src)
	}
	m := v.numbers
	lo, hi, err := fixedScratchPair(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lo.Close(), hi.Close()) }()
	lowerInf, upperInf := src.lowerInf, src.upperInf
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	if n%2 != 0 {
		// The source's odd branch uses the manager's ambient rounding.
		if !lowerInf {
			if err = m.Power(src.lower, n, lo); err != nil {
				return err
			}
		}
		if !upperInf {
			if err = m.Power(src.upper, n, hi); err != nil {
				return err
			}
		}
	} else {
		positive, e := (interval.FixedExtended{Numbers: m}).IsPos(src.lower, src.lowerKind())
		if e != nil {
			return e
		}
		negative, e := (interval.FixedExtended{Numbers: m}).IsNeg(src.upper, src.upperKind())
		if e != nil {
			return e
		}
		switch {
		case positive:
			if err = m.SetRounding(false); err != nil {
				return err
			}
			if err = m.Power(src.lower, n, lo); err != nil {
				return err
			}
			if !upperInf {
				if err = m.SetRounding(true); err != nil {
					return err
				}
				if err = m.Power(src.upper, n, hi); err != nil {
					return err
				}
			}
		case negative:
			if err = m.SetRounding(false); err != nil {
				return err
			}
			if err = m.Power(src.upper, n, lo); err != nil {
				return err
			}
			if !lowerInf {
				if err = m.SetRounding(true); err != nil {
					return err
				}
				if err = m.Power(src.lower, n, hi); err != nil {
					return err
				}
			}
			lowerInf, upperInf = false, lowerInf
			lowerOpen, upperOpen = upperOpen, lowerOpen
		default:
			left, right, e := fixedScratchPair(m)
			if e != nil {
				return e
			}
			defer func() { err = errors.Join(err, left.Close(), right.Close()) }()
			if !lowerInf {
				if err = m.Copy(left, src.lower); err != nil {
					return err
				}
			}
			if !upperInf {
				if err = m.Copy(right, src.upper); err != nil {
					return err
				}
			}
			if err = m.SetRounding(true); err != nil {
				return err
			}
			extended := interval.FixedExtended{Numbers: m}
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
	if err = m.Swap(v.lower, lo); err != nil {
		return err
	}
	if err = m.Swap(v.upper, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lowerInf, upperInf
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
