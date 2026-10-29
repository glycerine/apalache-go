package realclosure

// Source: src/math/realclosure/realclosure.cpp:3743-3770, 3881-3920.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// signApproxAtBinaryBound follows the source Horner interval sequence for
// retained algebraic-root and rational coefficient intervals. Rational
// coefficient refinement follows an ambiguous result. Exact fallback remains
// separate work.
func (s *realValueStore) signApproxAtBinaryBound(coefficients []*realValue, bounds *numeral.BinaryRationalManager, bound *numeral.BinaryRational) (sign int32, err error) {
	for _, value := range coefficients {
		if value == nil {
			continue
		}
		if value.store != s || value.deleted || value.refs == 0 || !value.isRational && (value.functionInterval == nil || value.functionInterval.numbers != bounds) {
			return 0, arena.ErrInvalidSlot
		}
	}
	coefficientInterval := func(value *realValue) (*BinaryInterval, error) {
		if !value.isRational {
			return value.functionInterval, nil
		}
		return s.cachedRationalApproxInterval(value, bounds)
	}
	point, err := NewBinaryInterval(bounds)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmInterval(point)) }()
	result, err := NewBinaryInterval(bounds)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmInterval(result)) }()
	if err = point.SetFinite(bound, bound, false, false); err != nil {
		return 0, err
	}
	evaluate := func() (int32, bool, error) {
		last, e := coefficientInterval(coefficients[len(coefficients)-1])
		if e != nil {
			return 0, false, e
		}
		if e = result.Mul(last, point); e != nil {
			return 0, false, e
		}
		for i := len(coefficients) - 2; i >= 0; i-- {
			if coefficients[i] != nil {
				coefficient, e := coefficientInterval(coefficients[i])
				if e != nil {
					return 0, false, e
				}
				if e = result.Add(result, coefficient); e != nil {
					return 0, false, e
				}
			}
			if i > 0 {
				if e = result.Mul(result, point); e != nil {
					return 0, false, e
				}
			}
		}
		contains, e := result.ContainsZero()
		if e != nil {
			return 0, false, e
		}
		if contains {
			// The source's exact fallback returns zero when every Horner
			// endpoint has collapsed to the same exact binary rational zero.
			// A nonzero singleton is already handled by the sign branch below.
			pointZero, e := result.isSingletonZero()
			if e != nil {
				return 0, false, e
			}
			if pointZero {
				return 0, true, nil
			}
			return 0, false, nil
		}
		positive, e := result.IsP()
		if e != nil {
			return 0, false, e
		}
		if positive {
			return 1, true, nil
		}
		return -1, true, nil
	}
	if sign, decided, e := evaluate(); e != nil || decided {
		return sign, e
	}
	// Source find_biggest_interval_magnitude chooses the initial precision.
	biggest := int32(-1 << 31)
	for _, value := range coefficients {
		if value == nil {
			continue
		}
		coefficient, e := coefficientInterval(value)
		if e != nil {
			return 0, e
		}
		m, e := s.sturmIntervalMagnitude(coefficient)
		if e != nil {
			return 0, e
		}
		if m > biggest {
			biggest = m
		}
	}
	precision := uint32(1)
	if biggest < 0 {
		precision = uint32(-int64(biggest))
	}
	for ; precision <= s.intervalMaxPrecision(); precision++ {
		for _, value := range coefficients {
			if value == nil {
				continue
			}
			if value.isRational {
				coefficient, e := coefficientInterval(value)
				if e != nil {
					return 0, e
				}
				if e := s.refineSturmRationalInterval(value, coefficient, precision); e != nil {
					return 0, e
				}
			} else {
				refined, e := s.refineSturmAlgebraicInterval(value, precision)
				if e != nil {
					return 0, e
				}
				if !refined {
					return 0, arena.ErrInvalidSlot
				}
			}
		}
		if sign, decided, e := evaluate(); e != nil || decided {
			return sign, e
		}
	}
	return 0, arena.ErrInvalidSlot
}
