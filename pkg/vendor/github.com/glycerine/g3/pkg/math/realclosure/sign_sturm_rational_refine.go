package realclosure

// Source: src/math/realclosure/realclosure.cpp:559-578, 900-910, 4073-4085.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// sturmIntervalMagnitude is the source upper magnitude of a finite interval's
// width. A singleton has the source INT_MIN sentinel.
func (s *realValueStore) sturmIntervalMagnitude(v *BinaryInterval) (m int32, err error) {
	if v == nil || v.lowerInf || v.upperInf {
		return 0, arena.ErrInvalidSlot
	}
	width := numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, s.releaseSturmBinary(v.numbers, &width)) }()
	if err = v.numbers.Sub(&v.upper, &v.lower, &width); err != nil {
		return 0, err
	}
	zero, err := v.numbers.IsZero(&width)
	if err != nil {
		return 0, err
	}
	if zero {
		return math.MinInt32, nil
	}
	return v.numbers.MagnitudeUpper(&width)
}

// refineSturmRationalInterval follows refine_rational_interval's strict width
// check and paired lower/upper updates, preserving the owned approximation.
func (s *realValueStore) refineSturmRationalInterval(value *realValue, v *BinaryInterval, precision uint32) (err error) {
	if s == nil || s.numbers == nil || !s.rationalOperand(value) || value == nil || v == nil || v.lowerInf || v.upperInf {
		return arena.ErrInvalidSlot
	}
	if !v.lowerOpen && !v.upperOpen {
		return nil
	}
	width := numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, s.releaseSturmBinary(v.numbers, &width)) }()
	for {
		if err = v.numbers.Sub(&v.upper, &v.lower, &width); err != nil {
			return err
		}
		precise, e := v.numbers.LessOneDiv2k(&width, precision)
		if e != nil {
			return e
		}
		if precise {
			return nil
		}
		if err = v.numbers.RefineLower(s.numbers, &value.rational, &v.lower, &v.upper); err != nil {
			return err
		}
		if err = v.numbers.RefineUpper(s.numbers, &value.rational, &v.lower, &v.upper); err != nil {
			return err
		}
	}
}
