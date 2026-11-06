package subpaving

// Source: src/math/interval/interval_def.h:839-925, concrete config_mpff.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// MulScalar preserves the source interval manager's zero-scalar reset.
func (v *FastInterval) MulScalar(k *numeral.FastFloat, src *FastInterval) error {
	return v.divMulScalar(k, src, false)
}

// DivScalar uses a separately rounded reciprocal for each output bound.
func (v *FastInterval) DivScalar(src *FastInterval, k *numeral.FastFloat) error {
	return v.divMulScalar(k, src, true)
}

func (v *FastInterval) divMulScalar(k *numeral.FastFloat, src *FastInterval, inverse bool) (err error) {
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
	loOwner, hiOwner, err := fastScratchPair(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, loOwner.Close(), hiOwner.Close()) }()
	lo, err := loOwner.Get()
	if err != nil {
		return err
	}
	hi, err := hiOwner.Get()
	if err != nil {
		return err
	}
	factor := k
	if inverse {
		invOwner, e := numeral.NewScopedFastFloat(m)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, invOwner.Close()) }()
		factor, e = invOwner.Get()
		if e != nil {
			return e
		}
	}
	e := interval.FastExtended{Numbers: m}
	lower, upper, err := src.endpoints()
	if err != nil {
		return err
	}
	lowerKind, lowerOpen := src.lowerKind(), src.lowerOpen
	upperKind, upperOpen := src.upperKind(), src.upperOpen
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
		if err = m.InvInPlace(factor); err != nil {
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
		if err = m.InvInPlace(factor); err != nil {
			return err
		}
	}
	uk, err := e.Mul(upper, upperKind, factor, interval.Finite, hi)
	if err != nil {
		return err
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
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
