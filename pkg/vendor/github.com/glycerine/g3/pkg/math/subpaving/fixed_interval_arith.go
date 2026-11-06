package subpaving

// Source: src/math/interval/interval_def.h:704-837, concrete config_mpfx.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func fixedScratchPair(m *numeral.FixedPointManager) (*numeral.FixedPoint, *numeral.FixedPoint, error) {
	lower, err := m.New()
	if err != nil {
		return nil, nil, err
	}
	upper, err := m.New()
	if err != nil {
		return nil, nil, errors.Join(err, lower.Close())
	}
	return lower, upper, nil
}

func (v *FixedInterval) Neg(src *FixedInterval) (err error) {
	if err = v.pair(src); err != nil {
		return err
	}
	lo, hi, err := fixedScratchPair(v.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lo.Close(), hi.Close()) }()
	e := interval.FixedExtended{Numbers: v.numbers}
	if !src.upperInf {
		if err = v.numbers.Copy(lo, src.upper); err != nil {
			return err
		}
	}
	if !src.lowerInf {
		if err = v.numbers.Copy(hi, src.lower); err != nil {
			return err
		}
	}
	lk, e1 := e.Neg(lo, src.upperKind())
	if e1 != nil {
		return e1
	}
	uk, e1 := e.Neg(hi, src.lowerKind())
	if e1 != nil {
		return e1
	}
	lowerOpen, upperOpen := src.upperOpen, src.lowerOpen
	if err = v.numbers.Swap(v.lower, lo); err != nil {
		return err
	}
	if err = v.numbers.Swap(v.upper, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}

func (v *FixedInterval) Add(a, b *FixedInterval) (err error) {
	if err = v.pair(a); err != nil {
		return err
	}
	if err = v.pair(b); err != nil {
		return err
	}
	lo, hi, err := fixedScratchPair(v.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lo.Close(), hi.Close()) }()
	e := interval.FixedExtended{Numbers: v.numbers}
	if err = v.numbers.SetRounding(false); err != nil {
		return err
	}
	lk, e1 := e.Add(a.lower, a.lowerKind(), b.lower, b.lowerKind(), lo)
	if e1 != nil {
		return e1
	}
	if err = v.numbers.SetRounding(true); err != nil {
		return err
	}
	uk, e1 := e.Add(a.upper, a.upperKind(), b.upper, b.upperKind(), hi)
	if e1 != nil {
		return e1
	}
	lowerOpen, upperOpen := a.lowerOpen || b.lowerOpen, a.upperOpen || b.upperOpen
	if err = v.numbers.Swap(v.lower, lo); err != nil {
		return err
	}
	if err = v.numbers.Swap(v.upper, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}

func (v *FixedInterval) Sub(a, b *FixedInterval) (err error) {
	if err = v.pair(a); err != nil {
		return err
	}
	if err = v.pair(b); err != nil {
		return err
	}
	lo, hi, err := fixedScratchPair(v.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lo.Close(), hi.Close()) }()
	e := interval.FixedExtended{Numbers: v.numbers}
	if err = v.numbers.SetRounding(false); err != nil {
		return err
	}
	lk, e1 := e.Sub(a.lower, a.lowerKind(), b.upper, b.upperKind(), lo)
	if e1 != nil {
		return e1
	}
	if err = v.numbers.SetRounding(true); err != nil {
		return err
	}
	uk, e1 := e.Sub(a.upper, a.upperKind(), b.lower, b.lowerKind(), hi)
	if e1 != nil {
		return e1
	}
	lowerOpen, upperOpen := a.lowerOpen || b.upperOpen, a.upperOpen || b.lowerOpen
	if err = v.numbers.Swap(v.lower, lo); err != nil {
		return err
	}
	if err = v.numbers.Swap(v.upper, hi); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == interval.MinusInfinity, uk == interval.PlusInfinity
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
