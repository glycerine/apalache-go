package subpaving

// Source: src/math/interval/interval_def.h:1390-1478, concrete config_mpfx.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// XnEqY computes the retained monomial inverse image x^n = y. Its concrete
// root owner supplies the source interval manager's resource checkpoint.
func (v *FixedInterval) XnEqY(src *FixedInterval, n uint32, precision *numeral.FixedPoint, roots interval.FixedRoot) (err error) {
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
	lower, upper, err := fixedScratchPair(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lower.Close(), upper.Close()) }()
	rootLower, rootUpper, err := fixedScratchPair(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rootLower.Close(), rootUpper.Close()) }()
	lowerInf, upperInf := src.lowerInf, src.upperInf
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	if n%2 == 0 {
		if err = roots.Bounds(src.upper, n, precision, rootLower, rootUpper); err != nil {
			return err
		}
		if err = v.traceFixedXnEqYBounds(src, rootLower, rootUpper, roots); err != nil {
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
		if err = m.SetRounding(false); err != nil {
			return err
		}
		if err = m.Copy(lower, rootUpper); err != nil {
			return err
		}
		if err = m.Neg(lower); err != nil {
			return err
		}
		lowerInf, upperInf = false, false
	} else {
		if !lowerInf {
			if err = roots.Bounds(src.lower, n, precision, rootLower, rootUpper); err != nil {
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
			if err = roots.Bounds(src.upper, n, precision, rootLower, rootUpper); err != nil {
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
	if err = m.Swap(v.lower, lower); err != nil {
		return err
	}
	if err = m.Swap(v.upper, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lowerInf, upperInf
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	if err = v.CheckInvariant(); err != nil {
		return err
	}
	if n%2 == 0 {
		return v.traceFixedXnEqYResult(roots)
	}
	return v.traceFixedNthRoot(src, roots)
}
