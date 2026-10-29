package interval

// Source: src/math/interval/interval_def.h:1503-1590, concrete mpq interval.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// Inv computes 1/src for an interval that excludes zero. Open zero bounds
// become infinite result bounds; infinite source bounds become open zeroes.
func (v *RationalInterval) Inv(src *RationalInterval) (err error) {
	if err = v.open(); err != nil {
		return err
	}
	if err = src.open(); err != nil {
		return err
	}
	if v.numbers != src.numbers {
		return ErrExtendedDomain
	}
	contains, err := src.ContainsZero()
	if err != nil {
		return err
	}
	if contains {
		return ErrExtendedDomain
	}
	m := v.numbers
	e := RationalExtended{Numbers: m}
	positive, err := e.IsPos(&src.lower, src.lowerKind())
	if err != nil {
		return err
	}
	lzero, err := e.IsZero(&src.lower, src.lowerKind())
	if err != nil {
		return err
	}
	negative, err := e.IsNeg(&src.upper, src.upperKind())
	if err != nil {
		return err
	}
	uzero, err := e.IsZero(&src.upper, src.upperKind())
	if err != nil {
		return err
	}
	if !positive && !(lzero && src.lowerOpen) && !negative && !(uzero && src.upperOpen) {
		return ErrExtendedDomain
	}
	lower, upper := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, m.Del(&lower), m.Del(&upper)) }()
	lo, uo := src.upperOpen, src.lowerOpen
	li, ui := false, false
	if positive || lzero && src.lowerOpen {
		if !src.upperInf {
			if err = m.Copy(&lower, &src.upper); err != nil {
				return err
			}
		}
		_, err = e.Inv(&lower, src.upperKind())
		if err != nil {
			return err
		}
		if lzero {
			ui, uo = true, true
		} else {
			if err = m.Inv(&src.lower, &upper); err != nil {
				return err
			}
		}
	} else {
		if !src.lowerInf {
			if err = m.Copy(&upper, &src.lower); err != nil {
				return err
			}
		}
		_, err = e.Inv(&upper, src.lowerKind())
		if err != nil {
			return err
		}
		if uzero {
			li, lo = true, true
		} else {
			if err = m.Inv(&src.upper, &lower); err != nil {
				return err
			}
		}
	}
	if err = m.Swap(&v.lower, &lower); err != nil {
		return err
	}
	if err = m.Swap(&v.upper, &upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = li, ui
	v.lowerOpen, v.upperOpen = lo, uo
	return v.CheckInvariant()
}
