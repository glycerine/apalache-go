package subpaving

// Source: src/math/interval/interval_def.h:839-925, concrete config_mpfx.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// MulScalar preserves the source interval manager's zero-scalar reset.
func (v *FixedInterval) MulScalar(k *numeral.FixedPoint, src *FixedInterval) error {
	return v.divMulScalar(k, src, false)
}

// DivScalar uses a separately rounded reciprocal for each output bound.
func (v *FixedInterval) DivScalar(src *FixedInterval, k *numeral.FixedPoint) error {
	return v.divMulScalar(k, src, true)
}

func (v *FixedInterval) divMulScalar(k *numeral.FixedPoint, src *FixedInterval, inverse bool) (err error) {
	if err = v.pair(src); err != nil {
		return err
	}
	m := v.numbers
	zero, err := m.IsZero(k)
	if err != nil {
		return err
	}
	if zero {
		return v.Reset()
	}
	positive, err := m.IsPos(k)
	if err != nil {
		return err
	}
	lo, hi, err := fixedScratchPair(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lo.Close(), hi.Close()) }()
	factor := k
	if inverse {
		inv, e := m.New()
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, inv.Close()) }()
		factor = inv
	}
	e := interval.FixedExtended{Numbers: m}
	lower, lowerKind, lowerOpen := src.lower, src.lowerKind(), src.lowerOpen
	upper, upperKind, upperOpen := src.upper, src.upperKind(), src.upperOpen
	if !positive {
		lower, upper = upper, lower
		lowerKind, upperKind = upperKind, lowerKind
		lowerOpen, upperOpen = upperOpen, lowerOpen
	}
	if err = m.SetRounding(false); err != nil {
		return err
	}
	if inverse {
		if err = m.Copy(factor, k); err != nil {
			return err
		}
		if err = m.Inv(factor); err != nil {
			return err
		}
	}
	lk, err := e.Mul(lower, lowerKind, factor, interval.Finite, lo)
	if err != nil {
		return err
	}
	if err = m.SetRounding(true); err != nil {
		return err
	}
	if inverse {
		if err = m.Copy(factor, k); err != nil {
			return err
		}
		if err = m.Inv(factor); err != nil {
			return err
		}
	}
	uk, err := e.Mul(upper, upperKind, factor, interval.Finite, hi)
	if err != nil {
		return err
	}
	if err = m.Swap(v.lower, lo); err != nil {
		return err
	}
	if err = m.Swap(v.upper, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
