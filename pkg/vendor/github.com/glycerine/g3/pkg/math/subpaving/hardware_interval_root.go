package subpaving

// Source: src/math/interval/interval_def.h:1390-1478, concrete config_hwf.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (v *HardwareInterval) LowerIsNeg() (bool, error) {
	lower, _, err := v.endpoints()
	if err != nil {
		return false, err
	}
	return (interval.HardwareExtended{Numbers: v.numbers}).IsNeg(lower, v.lowerKind()), nil
}

// NthRoot computes the source monotone interval root. Even degrees require a
// nonnegative lower bound; the inverse image of an even power is XnEqY.
func (v *HardwareInterval) NthRoot(src *HardwareInterval, n uint32, precision *numeral.HardwareFloat, roots interval.HardwareRoot) error {
	if err := v.writablePair(src); err != nil {
		return err
	}
	if roots.Numbers != v.numbers {
		return arena.ErrInvalidSlot
	}
	if n == 0 {
		return interval.ErrExtendedDomain
	}
	if n%2 == 0 {
		negative, err := src.LowerIsNeg()
		if err != nil {
			return err
		}
		if negative {
			return interval.ErrExtendedDomain
		}
	}
	if n == 1 {
		return v.SetFrom(src)
	}
	srcLower, srcUpper, _ := src.endpoints()
	var lower, upper, rootLower, rootUpper numeral.HardwareFloat
	lowerInf, upperInf := src.lowerInf, src.upperInf
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	if !lowerInf {
		if err := roots.Bounds(srcLower, n, precision, &rootLower, &rootUpper); err != nil {
			return err
		}
		lowerOpen = src.lowerOpen && v.numbers.Equal(&rootLower, &rootUpper)
		lower = rootLower
	} else {
		if n%2 == 0 {
			return interval.ErrExtendedDomain
		}
		lowerOpen = true
	}
	if !upperInf {
		if err := roots.Bounds(srcUpper, n, precision, &rootLower, &rootUpper); err != nil {
			return err
		}
		upperOpen = src.upperOpen && v.numbers.Equal(&rootLower, &rootUpper)
		upper = rootUpper
	} else {
		upperOpen = true
	}
	dstLower, dstUpper, _ := v.endpoints()
	*dstLower, *dstUpper = lower, upper
	v.lowerInf, v.upperInf = lowerInf, upperInf
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	if err := v.CheckInvariant(); err != nil {
		return err
	}
	v.traceNthRoot(src, roots)
	return nil
}

// XnEqY returns the interval inverse image of the retained monomial x^n = y.
func (v *HardwareInterval) XnEqY(src *HardwareInterval, n uint32, precision *numeral.HardwareFloat, roots interval.HardwareRoot) error {
	if err := v.writablePair(src); err != nil {
		return err
	}
	if roots.Numbers != v.numbers {
		return arena.ErrInvalidSlot
	}
	if n == 0 {
		return interval.ErrExtendedDomain
	}
	if n%2 != 0 {
		return v.NthRoot(src, n, precision, roots)
	}
	negative, err := src.LowerIsNeg()
	if err != nil {
		return err
	}
	if src.lowerInf || negative {
		return interval.ErrExtendedDomain
	}
	if src.upperInf {
		return v.Reset()
	}
	_, srcUpper, _ := src.endpoints()
	var rootLower, rootUpper, lower numeral.HardwareFloat
	if err := roots.Bounds(srcUpper, n, precision, &rootLower, &rootUpper); err != nil {
		return err
	}
	v.traceXnEqYBounds(src, &rootLower, &rootUpper, roots)
	open := src.upperOpen && v.numbers.Equal(&rootLower, &rootUpper)
	v.numbers.RoundToMinusInf()
	if err := v.numbers.Neg(&rootUpper, &lower); err != nil {
		return err
	}
	dstLower, dstUpper, _ := v.endpoints()
	*dstLower, *dstUpper = lower, rootUpper
	v.lowerInf, v.upperInf = false, false
	v.lowerOpen, v.upperOpen = open, open
	if err := v.CheckInvariant(); err != nil {
		return err
	}
	v.traceXnEqYResult(roots)
	return nil
}
