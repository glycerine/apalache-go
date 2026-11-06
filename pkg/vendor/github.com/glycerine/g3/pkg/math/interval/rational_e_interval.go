package interval

// Source: src/math/interval/interval_def.h:1961-2007.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// E encloses Euler's constant by the source factorial series and 4/(k+1)!
// remainder estimate.
func (r *RationalTranscendentals) E(k uint32, out *RationalInterval) (err error) {
	if err = r.validate(); err != nil || out == nil {
		if err != nil {
			return err
		}
		return arena.ErrInvalidSlot
	}
	if err = out.open(); err != nil || out.numbers != r.Numbers {
		if err != nil {
			return err
		}
		return arena.ErrInvalidSlot
	}
	s := rationalTranscendentalScratch{numbers: r.Numbers}
	defer func() { err = errors.Join(err, s.close()) }()
	lower, upper := s.new(), s.new()
	denominator, term, factor, remainder := s.new(), s.new(), s.new(), s.new()
	if err = r.eSeries(k, lower, denominator, term); err != nil {
		return err
	}
	if err = rationalFactorial(r.Numbers, k+1, denominator, factor); err != nil {
		return err
	}
	if err = r.Numbers.Inv(denominator, remainder); err != nil {
		return err
	}
	if err = r.Numbers.SetInt32(factor, 4); err != nil {
		return err
	}
	if err = r.Numbers.Mul(factor, remainder, remainder); err != nil {
		return err
	}
	if err = r.Numbers.Add(lower, remainder, upper); err != nil {
		return err
	}
	return out.SetFinite(lower, upper, false, false)
}
