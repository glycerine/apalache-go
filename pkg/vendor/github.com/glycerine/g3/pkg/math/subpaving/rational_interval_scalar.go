package subpaving

// Source: src/math/interval/interval_def.h:839-925, concrete config_mpq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// MulScalar follows div_mul's source zero-scalar reset and sign-dependent
// endpoint order. Exact rationals do not need a rounding-mode change.
func (v *RationalInterval) MulScalar(k *numeral.Rational, src *RationalInterval) error {
	return v.divMulScalar(k, src, false)
}

// DivScalar uses the source div_mul reciprocal path for each endpoint.
func (v *RationalInterval) DivScalar(src *RationalInterval, k *numeral.Rational) error {
	return v.divMulScalar(k, src, true)
}

func (v *RationalInterval) divMulScalar(k *numeral.Rational, src *RationalInterval, inverse bool) (err error) {
	slo, shi, err := v.pairView(src)
	if err != nil {
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
	lower, upper := rationalIntervalScratch()
	defer func() { err = errors.Join(err, m.Del(&lower), m.Del(&upper)) }()
	var inverseValue numeral.Rational
	factor := k
	if inverse {
		inverseValue = numeral.NewRational()
		factor = &inverseValue
		defer func() { err = errors.Join(err, m.Del(factor)) }()
	}
	e := interval.RationalExtended{Numbers: m}
	lowerValue, upperValue := slo, shi
	lowerKind, upperKind := src.lowerKind(), src.upperKind()
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	if !positive {
		lowerValue, upperValue = upperValue, lowerValue
		lowerKind, upperKind = upperKind, lowerKind
		lowerOpen, upperOpen = upperOpen, lowerOpen
	}
	if inverse {
		if err = m.Inv(k, factor); err != nil {
			return err
		}
	}
	lk, err := e.Mul(lowerValue, lowerKind, factor, interval.Finite, &lower)
	if err != nil {
		return err
	}
	if inverse {
		if err = m.Inv(k, factor); err != nil {
			return err
		}
	}
	uk, err := e.Mul(upperValue, upperKind, factor, interval.Finite, &upper)
	if err != nil {
		return err
	}
	if err = m.Swap(&v.lower, &lower); err != nil {
		return err
	}
	if err = m.Swap(&v.upper, &upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
