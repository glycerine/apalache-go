package interval

// Source: src/math/interval/interval_def.h:1873-1927.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// Pi stores the exact BBP partial sum and its source remainder bound in out.
func (r *RationalTranscendentals) Pi(n uint32, out *RationalInterval) (err error) {
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
	term, aux, scale := s.new(), s.new(), s.new()
	if err = r.Numbers.SetInt32(lower, 0); err != nil {
		return err
	}
	for i := uint32(0); i <= n; i++ {
		if err = r.piSeries(i, term, aux, scale); err != nil {
			return err
		}
		if err = r.Numbers.Add(lower, term, lower); err != nil {
			return err
		}
	}
	// len = (1/15) * (1/16)^n.
	if err = r.Numbers.SetFraction(scale, 1, 16); err != nil {
		return err
	}
	if err = r.Numbers.Power(scale, n, scale); err != nil {
		return err
	}
	if err = r.Numbers.SetFraction(aux, 1, 15); err != nil {
		return err
	}
	if err = r.Numbers.Mul(aux, scale, scale); err != nil {
		return err
	}
	if err = r.Numbers.Add(lower, scale, upper); err != nil {
		return err
	}
	return out.SetFinite(lower, upper, false, false)
}
