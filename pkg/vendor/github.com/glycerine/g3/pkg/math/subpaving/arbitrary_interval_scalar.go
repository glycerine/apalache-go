package subpaving

// Source: src/math/interval/interval_def.h:839-925, concrete config_mpf.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// MulScalar preserves the source interval manager's zero-scalar reset.
func (v *ArbitraryInterval) MulScalar(k *numeral.ArbitraryFloat, src *ArbitraryInterval) error {
	return v.divMulScalar(k, src, false)
}

// DivScalar uses a separately rounded reciprocal for each output bound.
func (v *ArbitraryInterval) DivScalar(src *ArbitraryInterval, k *numeral.ArbitraryFloat) error {
	return v.divMulScalar(k, src, true)
}

func (v *ArbitraryInterval) divMulScalar(k *numeral.ArbitraryFloat, src *ArbitraryInterval, inverse bool) (err error) {
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
	var lowerValue, upperValue numeral.ArbitraryFloat
	lo, hi := &lowerValue, &upperValue
	defer func() { err = errors.Join(err, m.Del(lo), m.Del(hi)) }()
	var inverseValue numeral.ArbitraryFloat
	factor := k
	if inverse {
		factor = &inverseValue
		defer func() { err = errors.Join(err, m.Del(factor)) }()
	}
	e := interval.ArbitraryExtended{Numbers: m}
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
	m.RoundToMinusInf()
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
	m.RoundToPlusInf()
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
