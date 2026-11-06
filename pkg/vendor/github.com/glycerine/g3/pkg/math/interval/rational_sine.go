package interval

// Source: src/math/interval/interval_def.h:437-501.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Sine returns exact rational Taylor enclosures. It preserves the source's
// odd-degree adjustment and conservative [-1,1] fallback.
func (r *RationalTranscendentals) Sine(a *numeral.Rational, k uint32, lower, upper *numeral.Rational) (err error) {
	if err = r.validate(); err != nil || a == nil || lower == nil || upper == nil || lower == upper {
		if err != nil {
			return err
		}
		return arena.ErrInvalidSlot
	}
	zero, err := r.Numbers.IsZero(a)
	if err != nil {
		return err
	}
	if zero {
		if err = r.Numbers.SetInt32(lower, 0); err != nil {
			return err
		}
		return r.Numbers.SetInt32(upper, 0)
	}
	if k%2 == 0 {
		k++
	}
	s := rationalTranscendentalScratch{numbers: r.Numbers}
	defer func() { err = errors.Join(err, s.close()) }()
	series, remainder := s.new(), s.new()
	power, factorial, factor := s.new(), s.new(), s.new()
	if err = r.Numbers.Copy(remainder, a); err != nil {
		return err
	}
	if err = r.Numbers.Abs(remainder); err != nil {
		return err
	}
	if err = r.Numbers.Power(remainder, k+1, remainder); err != nil {
		return err
	}
	if err = rationalFactorial(r.Numbers, k+1, factorial, factor); err != nil {
		return err
	}
	if err = r.Numbers.Div(remainder, factorial, remainder); err != nil {
		return err
	}
	if err = r.sineSeries(a, k, series, power, factorial, factor); err != nil {
		return err
	}
	if err = r.Numbers.Sub(series, remainder, lower); err != nil {
		return err
	}
	minusOne := s.new()
	if err = r.Numbers.SetInt32(minusOne, -1); err != nil {
		return err
	}
	comparison, err := r.Numbers.Compare(lower, minusOne)
	if err != nil {
		return err
	}
	if comparison < 0 {
		if err = r.Numbers.SetInt32(lower, -1); err != nil {
			return err
		}
		return r.Numbers.SetInt32(upper, 1)
	}
	return r.Numbers.Add(series, remainder, upper)
}
