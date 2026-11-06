package subpaving

// Source: src/math/interval/interval_def.h:1271-1378, concrete config_hwf.

import (
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Power retains the source positive, negative, crossing-zero and odd branches.
// Scalar scratch preserves the input when the destination aliases it.
func (v *HardwareInterval) Power(src *HardwareInterval, n uint32) error {
	if err := v.writablePair(src); err != nil {
		return err
	}
	if n == 1 {
		return v.SetFrom(src)
	}
	m := v.numbers
	e := interval.HardwareExtended{Numbers: m}
	srcLower, srcUpper, _ := src.endpoints()
	lowerInf, upperInf := src.lowerInf, src.upperInf
	lowerOpen, upperOpen := src.lowerOpen, src.upperOpen
	var lo, hi numeral.HardwareFloat
	if n%2 != 0 {
		// The source odd branch uses ambient rounding.
		if !lowerInf {
			if err := m.Power(srcLower, n, &lo); err != nil {
				return err
			}
		}
		if !upperInf {
			if err := m.Power(srcUpper, n, &hi); err != nil {
				return err
			}
		}
	} else {
		switch {
		case e.IsPos(srcLower, src.lowerKind()):
			m.RoundToMinusInf()
			if err := m.Power(srcLower, n, &lo); err != nil {
				return err
			}
			if !upperInf {
				m.RoundToPlusInf()
				if err := m.Power(srcUpper, n, &hi); err != nil {
					return err
				}
			}
		case e.IsNeg(srcUpper, src.upperKind()):
			m.RoundToMinusInf()
			if err := m.Power(srcUpper, n, &lo); err != nil {
				return err
			}
			if !lowerInf {
				m.RoundToPlusInf()
				if err := m.Power(srcLower, n, &hi); err != nil {
					return err
				}
			}
			lowerInf, upperInf = false, lowerInf
			lowerOpen, upperOpen = upperOpen, lowerOpen
		default:
			var left, right numeral.HardwareFloat
			if !lowerInf {
				if err := m.Set(&left, srcLower); err != nil {
					return err
				}
			}
			if !upperInf {
				if err := m.Set(&right, srcUpper); err != nil {
					return err
				}
			}
			m.RoundToPlusInf()
			lk, err := e.Power(&left, src.lowerKind(), n)
			if err != nil {
				return err
			}
			rk, err := e.Power(&right, src.upperKind(), n)
			if err != nil {
				return err
			}
			pickLeft := e.Greater(&left, lk, &right, rk)
			if !pickLeft {
				pickLeft = e.Equal(&left, lk, &right, rk) && !lowerOpen && upperOpen
			}
			if pickLeft {
				hi = left
				upperInf, upperOpen = lk == interval.PlusInfinity, lowerOpen
			} else {
				hi = right
				upperInf = rk == interval.PlusInfinity
			}
			if err = m.Reset(&lo); err != nil {
				return err
			}
			lowerInf, lowerOpen = false, false
		}
	}
	dstLower, dstUpper, _ := v.endpoints()
	*dstLower, *dstUpper = lo, hi
	v.lowerInf, v.upperInf = lowerInf, upperInf
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return v.CheckInvariant()
}
