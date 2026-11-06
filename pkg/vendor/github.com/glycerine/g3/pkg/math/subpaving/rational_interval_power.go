package subpaving

// Source: src/math/interval/interval_def.h:1271-1378, concrete config_mpq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Power follows the source positive, negative, crossing-zero and odd branches.
// Scratch endpoints keep input intact when dst aliases src.
func (v *RationalInterval) Power(src *RationalInterval, n uint32) (err error) {
	slo, shi, err := v.pairView(src)
	if err != nil {
		return err
	}
	if n == 1 {
		return v.SetFrom(src)
	}
	m := v.numbers
	lower, upper := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, m.Del(&lower), m.Del(&upper)) }()
	lo, uo := src.lowerOpen, src.upperOpen
	li, ui := src.lowerInf, src.upperInf
	if n%2 != 0 {
		if !li {
			if err = m.Power(slo, n, &lower); err != nil {
				return err
			}
		}
		if !ui {
			if err = m.Power(shi, n, &upper); err != nil {
				return err
			}
		}
	} else {
		extended := interval.RationalExtended{Numbers: m}
		positive, e := extended.IsPos(slo, src.lowerKind())
		if e != nil {
			return e
		}
		negative, e := extended.IsNeg(shi, src.upperKind())
		if e != nil {
			return e
		}
		switch {
		case !li && positive:
			if err = m.Power(slo, n, &lower); err != nil {
				return err
			}
			if !ui {
				if err = m.Power(shi, n, &upper); err != nil {
					return err
				}
			}
		case !ui && negative:
			if err = m.Power(shi, n, &lower); err != nil {
				return err
			}
			if !li {
				if err = m.Power(slo, n, &upper); err != nil {
					return err
				}
			}
			lo, uo = uo, lo
			li, ui = false, li
		default:
			// Source computes both powered extended endpoints and chooses the
			// larger. Equal values prefer a closed endpoint.
			left, right := numeral.NewRational(), numeral.NewRational()
			defer func() { err = errors.Join(err, m.Del(&left), m.Del(&right)) }()
			if !li {
				if err = m.Copy(&left, slo); err != nil {
					return err
				}
			}
			if !ui {
				if err = m.Copy(&right, shi); err != nil {
					return err
				}
			}
			e := interval.RationalExtended{Numbers: m}
			lk, rk := src.lowerKind(), src.upperKind()
			lk, err = e.Power(&left, lk, n)
			if err != nil {
				return err
			}
			rk, err = e.Power(&right, rk, n)
			if err != nil {
				return err
			}
			pickLeft, e1 := e.Greater(&left, lk, &right, rk)
			if e1 != nil {
				return e1
			}
			if !pickLeft {
				equal, e1 := e.Equal(&left, lk, &right, rk)
				if e1 != nil {
					return e1
				}
				pickLeft = equal && !lo && uo
			}
			if pickLeft {
				if err = m.Swap(&upper, &left); err != nil {
					return err
				}
				ui, uo = lk == interval.PlusInfinity, lo
			} else {
				if err = m.Swap(&upper, &right); err != nil {
					return err
				}
				ui = rk == interval.PlusInfinity
			}
			li, lo = false, false
		}
	}
	if li {
		if err = m.Reset(&lower); err != nil {
			return err
		}
		lo = true
	}
	if ui {
		if err = m.Reset(&upper); err != nil {
			return err
		}
		uo = true
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
