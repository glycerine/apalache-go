package subpaving

// Source: src/math/interval/interval_def.h:839-925, concrete config_hwf.

import (
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (v *HardwareInterval) MulScalar(k *numeral.HardwareFloat, src *HardwareInterval) error {
	return v.divMulScalar(k, src, false)
}
func (v *HardwareInterval) DivScalar(src *HardwareInterval, k *numeral.HardwareFloat) error {
	return v.divMulScalar(k, src, true)
}
func (v *HardwareInterval) divMulScalar(k *numeral.HardwareFloat, src *HardwareInterval, inverse bool) error {
	if err := v.writablePair(src); err != nil {
		return err
	}
	m := v.numbers
	if m.IsZero(k) {
		return v.Reset()
	}
	positive := m.IsPos(k)
	lower, upper, _ := src.endpoints()
	lowerKind, upperKind := src.lowerKind(), src.upperKind()
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	if !positive {
		lower, upper = upper, lower
		lowerKind, upperKind = upperKind, lowerKind
		lowerOpen, upperOpen = upperOpen, lowerOpen
	}
	var lo, hi, factor numeral.HardwareFloat
	e := interval.HardwareExtended{Numbers: m}
	m.RoundToMinusInf()
	operand := k
	if inverse {
		if err := m.Set(&factor, k); err != nil {
			return err
		}
		if err := m.Inv(&factor); err != nil {
			return err
		}
		operand = &factor
	}
	lk, err := e.Mul(lower, lowerKind, operand, interval.Finite, &lo)
	if err != nil {
		return err
	}
	m.RoundToPlusInf()
	if inverse {
		if err = m.Set(&factor, k); err != nil {
			return err
		}
		if err = m.Inv(&factor); err != nil {
			return err
		}
	}
	uk, err := e.Mul(upper, upperKind, operand, interval.Finite, &hi)
	if err != nil {
		return err
	}
	dstLower, dstUpper, _ := v.endpoints()
	*dstLower, *dstUpper = lo, hi
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
