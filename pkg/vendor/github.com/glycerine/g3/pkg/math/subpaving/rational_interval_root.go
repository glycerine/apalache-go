package subpaving

// Source: src/math/interval/interval_def.h:1390-1478, concrete config_mpq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (v *RationalInterval) LowerIsNeg() (bool, error) {
	lower, _, err := v.resolved()
	if err != nil {
		return false, err
	}
	return (interval.RationalExtended{Numbers: v.numbers}).IsNeg(lower, v.lowerKind())
}

// XnEqY encloses the inverse image of x^n = y. Even degrees use the upper
// root to bound both signs; odd degrees preserve each endpoint's direction.
func (v *RationalInterval) XnEqY(src *RationalInterval, n uint32, precision *numeral.Rational, roots interval.RationalRoot) (err error) {
	slo, shi, err := v.pairView(src)
	if err != nil {
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
	}
	lower, upper := numeral.NewRational(), numeral.NewRational()
	rootLower, rootUpper := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, m.Del(&rootUpper), m.Del(&rootLower), m.Del(&upper), m.Del(&lower)) }()
	lowerInf, upperInf := src.lowerInf, src.upperInf
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	if n%2 == 0 {
		if err = roots.Bounds(shi, n, precision, &rootLower, &rootUpper); err != nil {
			return err
		}
		if err = v.traceRationalXnEqYBounds(src, &rootLower, &rootUpper, roots); err != nil {
			return err
		}
		equal, err := m.Equal(&rootLower, &rootUpper)
		if err != nil {
			return err
		}
		lowerOpen, upperOpen = src.upperOpen && equal, src.upperOpen && equal
		if err = m.Copy(&upper, &rootUpper); err != nil {
			return err
		}
		if err = m.Copy(&lower, &rootUpper); err != nil {
			return err
		}
		if err = m.Neg(&lower); err != nil {
			return err
		}
		lowerInf, upperInf = false, false
	} else {
		if !lowerInf {
			if err = roots.Bounds(slo, n, precision, &rootLower, &rootUpper); err != nil {
				return err
			}
			equal, err := m.Equal(&rootLower, &rootUpper)
			if err != nil {
				return err
			}
			lowerOpen = src.lowerOpen && equal
			if err = m.Copy(&lower, &rootLower); err != nil {
				return err
			}
		} else {
			lowerOpen = true
		}
		if !upperInf {
			if err = roots.Bounds(shi, n, precision, &rootLower, &rootUpper); err != nil {
				return err
			}
			equal, err := m.Equal(&rootLower, &rootUpper)
			if err != nil {
				return err
			}
			upperOpen = src.upperOpen && equal
			if err = m.Copy(&upper, &rootUpper); err != nil {
				return err
			}
		} else {
			upperOpen = true
		}
	}
	if err = m.Swap(&v.lower, &lower); err != nil {
		return err
	}
	if err = m.Swap(&v.upper, &upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lowerInf, upperInf
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	if err = v.CheckInvariant(); err != nil {
		return err
	}
	if n%2 == 0 {
		return v.traceRationalXnEqYResult(roots)
	}
	return v.traceRationalNthRoot(src, roots)
}
