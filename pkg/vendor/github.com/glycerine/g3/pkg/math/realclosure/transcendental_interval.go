package realclosure

// Source: src/math/realclosure/realclosure.cpp:1302-1330 and 2518-2542.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// piInterval creates the source's first binary enclosure for pi.
func (v *APIValues) piInterval() (result *BinaryInterval, err error) {
	return v.predefinedTranscendentalInterval(true)
}

// eInterval creates the source's first binary enclosure for Euler's number.
func (v *APIValues) eInterval() (result *BinaryInterval, err error) {
	return v.predefinedTranscendentalInterval(false)
}

func (v *APIValues) predefinedTranscendentalInterval(pi bool) (result *BinaryInterval, err error) {
	if v == nil || v.transcendentals == nil || v.store.numbers == nil || v.initialPrecision == 0 {
		return nil, arena.ErrInvalidSlot
	}
	rational, err := interval.NewRationalInterval(v.store.numbers)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rational.Close()) }()
	if pi {
		err = v.transcendentals.Pi(1, rational)
	} else {
		err = v.transcendentals.E(1, rational)
	}
	if err != nil {
		return nil, err
	}
	return v.rationalIntervalToBinary(rational, v.initialPrecision)
}

// infinitesimalInterval mirrors mk_infinitesimal's initial interval
// (0, 1/2^initial_precision); value arithmetic over that extension is ported
// separately from interval creation.
func (v *APIValues) infinitesimalInterval() (result *BinaryInterval, err error) {
	if v == nil || v.bounds == nil || v.infPrecision == 0 {
		return nil, arena.ErrInvalidSlot
	}
	lower, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, v.bounds.Del(&lower), v.bounds.Del(&upper)) }()
	if err = v.bounds.SetInt32(&lower, 0); err != nil {
		return nil, err
	}
	if err = v.bounds.SetInt32Power(&upper, 1, v.infPrecision); err != nil {
		return nil, err
	}
	result, err = NewBinaryInterval(v.bounds)
	if err != nil {
		return nil, err
	}
	if err = result.SetFinite(&lower, &upper, true, true); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

// rationalPointInterval follows mpq_to_mpbqi: start with the source dyadic
// truncation, bracket it by doubling, and bisect until the interval meets the
// requested precision and excludes zero.
func (v *APIValues) rationalPointInterval(q *numeral.Rational, precision uint32) (result *BinaryInterval, err error) {
	if v == nil || v.bounds == nil || v.transcendentals == nil || q == nil || precision == 0 {
		return nil, arena.ErrInvalidSlot
	}
	lo, hi := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, v.bounds.Del(&lo), v.bounds.Del(&hi)) }()
	exact, err := v.bounds.FromRational(v.transcendentals.Numbers, q, &lo)
	if err != nil {
		return nil, err
	}
	if err = v.bounds.Copy(&hi, &lo); err != nil {
		return nil, err
	}
	if !exact {
		if err = v.bounds.Mul2(&hi); err != nil {
			return nil, err
		}
		negative, e := v.transcendentals.Numbers.IsNeg(q)
		if e != nil {
			return nil, e
		}
		if negative {
			mid := numeral.NewBinaryRational()
			if err = v.bounds.Copy(&mid, &lo); err != nil {
				return nil, errors.Join(err, v.bounds.Del(&mid))
			}
			if err = v.bounds.Copy(&lo, &hi); err == nil {
				err = v.bounds.Copy(&hi, &mid)
			}
			err = errors.Join(err, v.bounds.Del(&mid))
			if err != nil {
				return nil, err
			}
		}
	}
	result, err = NewBinaryInterval(v.bounds)
	if err != nil {
		return nil, err
	}
	cleanup := func(cause error) (*BinaryInterval, error) {
		return nil, errors.Join(cause, result.Close())
	}
	if err = result.SetFinite(&lo, &hi, !exact, !exact); err != nil {
		return cleanup(err)
	}
	if exact {
		return result, nil
	}
	for {
		containsZero, e := result.ContainsZero()
		if e != nil {
			return cleanup(e)
		}
		precise, e := binaryIntervalPrecise(result, precision)
		if e != nil {
			return cleanup(e)
		}
		lowerZero, e := v.bounds.IsZero(&result.lower)
		if e != nil {
			return cleanup(e)
		}
		upperZero, e := v.bounds.IsZero(&result.upper)
		if e != nil {
			return cleanup(e)
		}
		if !containsZero && precise && !lowerZero && !upperZero {
			return result, nil
		}
		if err = v.bounds.RefineLower(v.transcendentals.Numbers, q, &result.lower, &result.upper); err != nil {
			return cleanup(err)
		}
		if err = v.bounds.RefineUpper(v.transcendentals.Numbers, q, &result.lower, &result.upper); err != nil {
			return cleanup(err)
		}
	}
}

// rationalIntervalToBinary converts both finite rational endpoints with the
// source's directed interval approximation, preserving endpoint openness.
func (v *APIValues) rationalIntervalToBinary(source *interval.RationalInterval, precision uint32) (result *BinaryInterval, err error) {
	if v == nil || source == nil || precision == 0 {
		return nil, arena.ErrInvalidSlot
	}
	lower, upper, lowerOpen, upperOpen, err := source.BorrowFiniteBounds()
	if err != nil {
		return nil, err
	}
	mag, err := source.Magnitude()
	if err != nil {
		return nil, err
	}
	if mag < 0 && uint32(-int64(mag)) < 1<<16 {
		precision = uint32(-int64(mag)) + 8
	}
	lo, err := v.rationalPointInterval(lower, precision)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, lo.Close()) }()
	hi, err := v.rationalPointInterval(upper, precision)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, hi.Close()) }()
	result, err = NewBinaryInterval(v.bounds)
	if err != nil {
		return nil, err
	}
	if err = result.SetFinite(&lo.lower, &hi.upper, lowerOpen || lo.lowerOpen, upperOpen || hi.upperOpen); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}
