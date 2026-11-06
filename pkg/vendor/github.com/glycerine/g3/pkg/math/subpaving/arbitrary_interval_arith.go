package subpaving

// Source: src/math/interval/interval_def.h:704-837, concrete config_mpf.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (v *ArbitraryInterval) pair(other *ArbitraryInterval) error {
	if err := v.mutable(); err != nil {
		return err
	}
	if _, _, err := other.endpoints(); err != nil {
		return err
	}
	if v.numbers != other.numbers {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (v *ArbitraryInterval) Neg(src *ArbitraryInterval) (err error) {
	if err = v.pair(src); err != nil {
		return err
	}
	var lower, upper numeral.ArbitraryFloat
	lo, hi := &lower, &upper
	defer func() { err = errors.Join(err, v.numbers.Del(lo), v.numbers.Del(hi)) }()
	slo, shi, err := src.endpoints()
	if err != nil {
		return err
	}
	e := interval.ArbitraryExtended{Numbers: v.numbers}
	if !src.upperInf {
		if err = v.numbers.Copy(lo, shi); err != nil {
			return err
		}
	}
	if !src.lowerInf {
		if err = v.numbers.Copy(hi, slo); err != nil {
			return err
		}
	}
	lk, err := e.Neg(lo, src.upperKind())
	if err != nil {
		return err
	}
	uk, err := e.Neg(hi, src.lowerKind())
	if err != nil {
		return err
	}
	loDst, hiDst, err := v.endpoints()
	if err != nil {
		return err
	}
	if err = v.numbers.Swap(loDst, lo); err != nil {
		return err
	}
	if err = v.numbers.Swap(hiDst, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = src.upperOpen, src.lowerOpen
	return v.CheckInvariant()
}

func (v *ArbitraryInterval) Add(a, b *ArbitraryInterval) (err error) {
	if err = v.pair(a); err != nil {
		return err
	}
	if err = v.pair(b); err != nil {
		return err
	}
	var lower, upper numeral.ArbitraryFloat
	lo, hi := &lower, &upper
	defer func() { err = errors.Join(err, v.numbers.Del(lo), v.numbers.Del(hi)) }()
	alo, ahi, err := a.endpoints()
	if err != nil {
		return err
	}
	blo, bhi, err := b.endpoints()
	if err != nil {
		return err
	}
	e := interval.ArbitraryExtended{Numbers: v.numbers}
	v.numbers.RoundToMinusInf()
	lk, err := e.Add(alo, a.lowerKind(), blo, b.lowerKind(), lo)
	if err != nil {
		return err
	}
	v.numbers.RoundToPlusInf()
	uk, err := e.Add(ahi, a.upperKind(), bhi, b.upperKind(), hi)
	if err != nil {
		return err
	}
	lowerOpen, upperOpen := a.lowerOpen || b.lowerOpen, a.upperOpen || b.upperOpen
	loDst, hiDst, err := v.endpoints()
	if err != nil {
		return err
	}
	if err = v.numbers.Swap(loDst, lo); err != nil {
		return err
	}
	if err = v.numbers.Swap(hiDst, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}

func (v *ArbitraryInterval) Sub(a, b *ArbitraryInterval) (err error) {
	if err = v.pair(a); err != nil {
		return err
	}
	if err = v.pair(b); err != nil {
		return err
	}
	var lower, upper numeral.ArbitraryFloat
	lo, hi := &lower, &upper
	defer func() { err = errors.Join(err, v.numbers.Del(lo), v.numbers.Del(hi)) }()
	alo, ahi, err := a.endpoints()
	if err != nil {
		return err
	}
	blo, bhi, err := b.endpoints()
	if err != nil {
		return err
	}
	e := interval.ArbitraryExtended{Numbers: v.numbers}
	v.numbers.RoundToMinusInf()
	lk, err := e.Sub(alo, a.lowerKind(), bhi, b.upperKind(), lo)
	if err != nil {
		return err
	}
	v.numbers.RoundToPlusInf()
	uk, err := e.Sub(ahi, a.upperKind(), blo, b.lowerKind(), hi)
	if err != nil {
		return err
	}
	lowerOpen, upperOpen := a.lowerOpen || b.upperOpen, a.upperOpen || b.lowerOpen
	loDst, hiDst, err := v.endpoints()
	if err != nil {
		return err
	}
	if err = v.numbers.Swap(loDst, lo); err != nil {
		return err
	}
	if err = v.numbers.Swap(hiDst, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
