package realclosure

// Source: src/math/interval/interval_def.h:1516-1587, concrete mpbq interval.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Inv computes a directed binary-rational enclosure for 1/src. Precision is
// the source real-closure numeral manager's division precision in bits.
func (v *BinaryInterval) Inv(src *BinaryInterval, precision uint32) (err error) {
	if err = v.pair(src); err != nil {
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
	e := interval.BinaryExtended{Numbers: m}
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
		return interval.ErrExtendedDomain
	}
	lower, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, m.Del(&lower), m.Del(&upper)) }()
	lo, uo := src.upperOpen, src.lowerOpen
	li, ui := false, false
	if positive || lzero && src.lowerOpen {
		if !src.upperInf {
			if err = m.Copy(&lower, &src.upper); err != nil {
				return err
			}
		}
		_, err = (interval.BinaryExtended{Numbers: m, DivisionPrecision: precision}).Inv(&lower, src.upperKind())
		if err != nil {
			return err
		}
		if lzero {
			ui, uo = true, true
		} else {
			one := numeral.NewBinaryInt(1)
			if err = m.ApproxDiv(&one, &src.lower, &upper, precision, true); err != nil {
				return err
			}
		}
	} else {
		if !src.lowerInf {
			if err = m.Copy(&upper, &src.lower); err != nil {
				return err
			}
		}
		_, err = (interval.BinaryExtended{Numbers: m, DivisionPrecision: precision, RoundUp: true}).Inv(&upper, src.lowerKind())
		if err != nil {
			return err
		}
		if uzero {
			li, lo = true, true
		} else {
			one := numeral.NewBinaryInt(1)
			if err = m.ApproxDiv(&one, &src.upper, &lower, precision, false); err != nil {
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
