package interval

// Source: src/math/interval/interval_def.h:1929-1943.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (r *RationalTranscendentals) newCachedInterval() (*RationalInterval, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	return NewRationalInterval(r.Numbers)
}

// SetPiPrecision refreshes pi and its three source scalar caches atomically.
func (r *RationalTranscendentals) SetPiPrecision(n uint32) (err error) {
	if n == 0 {
		return arena.ErrInvalidSlot
	}
	pi, e := r.newCachedInterval()
	if e != nil {
		return e
	}
	half, e := r.newCachedInterval()
	if e != nil {
		return errors.Join(e, pi.Close())
	}
	threeHalf, e := r.newCachedInterval()
	if e != nil {
		return errors.Join(e, half.Close(), pi.Close())
	}
	twice, e := r.newCachedInterval()
	if e != nil {
		return errors.Join(e, threeHalf.Close(), half.Close(), pi.Close())
	}
	cleanup := func(cause error) error {
		return errors.Join(cause, twice.Close(), threeHalf.Close(), half.Close(), pi.Close())
	}
	if e = r.Pi(n, pi); e != nil {
		return cleanup(e)
	}
	scalar := numeral.NewRational()
	defer func() { err = errors.Join(err, r.Numbers.Del(&scalar)) }()
	for _, item := range []struct {
		numerator, denominator int64
		destination            *RationalInterval
	}{{1, 2, half}, {3, 2, threeHalf}, {2, 1, twice}} {
		if e = r.Numbers.SetFraction(&scalar, item.numerator, uint64(item.denominator)); e != nil {
			return cleanup(e)
		}
		if e = item.destination.MulScalar(&scalar, pi); e != nil {
			return cleanup(e)
		}
	}
	if e = r.Close(); e != nil {
		return cleanup(e)
	}
	r.piPrecision, r.piHalf, r.pi, r.threePiHalf, r.twoPi = n, half, pi, threeHalf, twice
	return nil
}

func (r *RationalTranscendentals) SetPiAtLeastPrecision(n uint32) error {
	if err := r.validate(); err != nil {
		return err
	}
	if n > r.piPrecision {
		return r.SetPiPrecision(n)
	}
	return nil
}

func (r *RationalTranscendentals) PiPrecision() uint32 { return r.piPrecision }
