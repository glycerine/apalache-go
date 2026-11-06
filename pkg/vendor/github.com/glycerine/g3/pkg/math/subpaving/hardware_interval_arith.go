package subpaving

// Source: src/math/interval/interval_def.h:704-837, concrete config_hwf.

import (
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (v *HardwareInterval) Neg(src *HardwareInterval) error {
	if err := v.writablePair(src); err != nil {
		return err
	}
	slo, shi, _ := src.endpoints()
	m := v.numbers
	var lower, upper numeral.HardwareFloat
	if !src.upperInf {
		if err := m.Set(&lower, shi); err != nil {
			return err
		}
	}
	if !src.lowerInf {
		if err := m.Set(&upper, slo); err != nil {
			return err
		}
	}
	e := interval.HardwareExtended{Numbers: m}
	lk, err := e.Neg(&lower, src.upperKind())
	if err != nil {
		return err
	}
	uk, err := e.Neg(&upper, src.lowerKind())
	if err != nil {
		return err
	}
	lo, hi, _ := v.endpoints()
	*lo, *hi = lower, upper
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = src.upperOpen, src.lowerOpen
	return v.CheckInvariant()
}
func (v *HardwareInterval) Add(a, b *HardwareInterval) error {
	if err := v.writablePair(a); err != nil {
		return err
	}
	if err := v.pair(b); err != nil {
		return err
	}
	alo, ahi, _ := a.endpoints()
	blo, bhi, _ := b.endpoints()
	m := v.numbers
	e := interval.HardwareExtended{Numbers: m}
	var lower, upper numeral.HardwareFloat
	m.RoundToMinusInf()
	lk, err := e.Add(alo, a.lowerKind(), blo, b.lowerKind(), &lower)
	if err != nil {
		return err
	}
	m.RoundToPlusInf()
	uk, err := e.Add(ahi, a.upperKind(), bhi, b.upperKind(), &upper)
	if err != nil {
		return err
	}
	lo, hi, _ := v.endpoints()
	*lo, *hi = lower, upper
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = a.lowerOpen || b.lowerOpen, a.upperOpen || b.upperOpen
	return v.CheckInvariant()
}
func (v *HardwareInterval) Sub(a, b *HardwareInterval) error {
	if err := v.writablePair(a); err != nil {
		return err
	}
	if err := v.pair(b); err != nil {
		return err
	}
	alo, ahi, _ := a.endpoints()
	blo, bhi, _ := b.endpoints()
	m := v.numbers
	e := interval.HardwareExtended{Numbers: m}
	var lower, upper numeral.HardwareFloat
	m.RoundToMinusInf()
	lk, err := e.Sub(alo, a.lowerKind(), bhi, b.upperKind(), &lower)
	if err != nil {
		return err
	}
	m.RoundToPlusInf()
	uk, err := e.Sub(ahi, a.upperKind(), blo, b.lowerKind(), &upper)
	if err != nil {
		return err
	}
	lo, hi, _ := v.endpoints()
	*lo, *hi = lower, upper
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = a.lowerOpen || b.upperOpen, a.upperOpen || b.lowerOpen
	return v.CheckInvariant()
}
