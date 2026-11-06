package subpaving

// Source: src/math/interval/interval_def.h:1510-1590, concrete config_mpq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Inv computes the source interval reciprocal. Open zero endpoints produce an
// infinite endpoint; an infinite input endpoint produces an open zero.
func (v *RationalInterval) Inv(src *RationalInterval) (err error) {
	slo, shi, err := v.pairView(src)
	if err != nil {
		return err
	}
	contains, err := src.ContainsZero()
	if err != nil {
		return err
	}
	if contains {
		return interval.ErrExtendedDomain
	}
	m := v.numbers
	e := interval.RationalExtended{Numbers: m}
	positive, err := e.IsPos(slo, src.lowerKind())
	if err != nil {
		return err
	}
	lzero, err := e.IsZero(slo, src.lowerKind())
	if err != nil {
		return err
	}
	negative, err := e.IsNeg(shi, src.upperKind())
	if err != nil {
		return err
	}
	uzero, err := e.IsZero(shi, src.upperKind())
	if err != nil {
		return err
	}
	if !positive && !(lzero && src.lowerOpen) && !negative && !(uzero && src.upperOpen) {
		return interval.ErrExtendedDomain
	}
	lower, upper := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, m.Del(&lower), m.Del(&upper)) }()
	lo, uo := src.upperOpen, src.lowerOpen
	li, ui := false, false
	if positive || lzero && src.lowerOpen {
		if !src.upperInf {
			if err = m.Copy(&lower, shi); err != nil {
				return err
			}
		}
		if _, err = e.Inv(&lower, src.upperKind()); err != nil {
			return err
		}
		if lzero {
			ui, uo = true, true
		} else if err = m.Inv(slo, &upper); err != nil {
			return err
		}
	} else {
		if !src.lowerInf {
			if err = m.Copy(&upper, slo); err != nil {
				return err
			}
		}
		if _, err = e.Inv(&upper, src.lowerKind()); err != nil {
			return err
		}
		if uzero {
			li, lo = true, true
		} else if err = m.Inv(shi, &lower); err != nil {
			return err
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
