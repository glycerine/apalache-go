package subpaving

// Source: src/math/interval/interval_def.h:1390-1478, concrete config_mpf.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (v *ArbitraryInterval) LowerIsNeg() (bool, error) {
	lower, _, err := v.endpoints()
	if err != nil {
		return false, err
	}
	return (interval.ArbitraryExtended{Numbers: v.numbers}).IsNeg(lower, v.lowerKind())
}

// XnEqY computes the retained monomial inverse image x^n = y. Its concrete
// root owner supplies the source interval manager's resource checkpoint.
func (v *ArbitraryInterval) XnEqY(src *ArbitraryInterval, n uint32, precision *numeral.ArbitraryFloat, roots interval.ArbitraryRoot) (err error) {
	if err = v.pair(src); err != nil {
		return err
	}
	if roots.Numbers != v.numbers {
		return arena.ErrInvalidSlot
	}
	if n == 0 {
		return interval.ErrExtendedDomain
	}
	if n == 1 {
		return v.SetFrom(src)
	}
	m := v.numbers
	srcLower, srcUpper, err := src.endpoints()
	if err != nil {
		return err
	}
	if n%2 == 0 {
		negative, e := src.LowerIsNeg()
		if e != nil {
			return e
		}
		if src.lowerInf || negative {
			return interval.ErrExtendedDomain
		}
		if src.upperInf {
			return v.Reset()
		}
	}
	var lowerValue, upperValue, rootLowerValue, rootUpperValue numeral.ArbitraryFloat
	lower, upper := &lowerValue, &upperValue
	rootLower, rootUpper := &rootLowerValue, &rootUpperValue
	defer func() { err = errors.Join(err, m.Del(lower), m.Del(upper), m.Del(rootLower), m.Del(rootUpper)) }()
	dstLower, dstUpper, err := v.endpoints()
	if err != nil {
		return err
	}
	lowerInf, upperInf := src.lowerInf, src.upperInf
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	if n%2 == 0 {
		if err = roots.Bounds(srcUpper, n, precision, rootLower, rootUpper); err != nil {
			return err
		}
		if err = v.traceXnEqYBounds(src, rootLower, rootUpper, roots); err != nil {
			return err
		}
		equal, e := m.Equal(rootLower, rootUpper)
		if e != nil {
			return e
		}
		lowerOpen, upperOpen = src.upperOpen && equal, src.upperOpen && equal
		if err = m.Copy(upper, rootUpper); err != nil {
			return err
		}
		m.SetRounding(false)
		if err = m.Copy(lower, rootUpper); err != nil {
			return err
		}
		if err = m.Neg(lower, lower); err != nil {
			return err
		}
		lowerInf, upperInf = false, false
	} else {
		if !lowerInf {
			if err = roots.Bounds(srcLower, n, precision, rootLower, rootUpper); err != nil {
				return err
			}
			equal, e := m.Equal(rootLower, rootUpper)
			if e != nil {
				return e
			}
			lowerOpen = src.lowerOpen && equal
			if err = m.Copy(lower, rootLower); err != nil {
				return err
			}
		} else {
			lowerOpen = true
		}
		if !upperInf {
			if err = roots.Bounds(srcUpper, n, precision, rootLower, rootUpper); err != nil {
				return err
			}
			equal, e := m.Equal(rootLower, rootUpper)
			if e != nil {
				return e
			}
			upperOpen = src.upperOpen && equal
			if err = m.Copy(upper, rootUpper); err != nil {
				return err
			}
		} else {
			upperOpen = true
		}
	}
	if err = m.Swap(dstLower, lower); err != nil {
		return err
	}
	if err = m.Swap(dstUpper, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lowerInf, upperInf
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	if err = v.CheckInvariant(); err != nil {
		return err
	}
	if n%2 == 0 {
		return v.traceXnEqYResult(roots)
	}
	return v.traceNthRoot(src, roots)
}
