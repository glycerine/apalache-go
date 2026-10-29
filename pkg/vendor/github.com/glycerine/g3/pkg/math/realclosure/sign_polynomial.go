package realclosure

// Source: src/math/realclosure/realclosure.cpp:155-175, 752-776,
// 864-889, 1120-1127, and 2557-2576.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

// realValueStore owns the retained rational and algebraic-root value variants.
// Other rational-function and extension cases remain to be ported.
type realValueStore struct {
	numbers                  *numeral.RationalManager
	initialPrecision         uint32
	initialPrecisionBound    bool
	maxPrecision             uint32
	maxPrecisionBound        bool
	pendingPrecisionHelper   *params.ModuleParamsOwner
	pendingSequence          []realPolynomial
	pendingSturmCoefficients [][]*realValue
	pendingSturmRationals    []numeral.Rational
	pendingSturmIntegers     []numeral.Int
	pendingSturmBinaries     []sturmBinaryTemp
	pendingSturmIntervals    []*BinaryInterval
}

// realValue is a retained nonzero rational or algebraic-root function value.
// A nil pointer is the source representation of zero. The creator initially
// owns one reference, which can be transferred to a polynomial or numeral.
type realValue struct {
	store             *realValueStore
	rational          numeral.Rational
	rationalInterval  *BinaryInterval
	functionNumerator realPolynomial
	functionInterval  *BinaryInterval
	root              *algebraicRoot
	refs              uint32
	isRational        bool
	deleted           bool
}

func (s *realValueStore) newRational(numerator int64, denominator uint64) (*realValue, error) {
	if s == nil || s.numbers == nil || denominator == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if numerator == 0 {
		return nil, nil
	}
	v := &realValue{store: s, rational: numeral.NewRational(), refs: 1, isRational: true}
	if err := s.numbers.SetFraction(&v.rational, numerator, denominator); err != nil {
		_ = s.releaseSturmRational(&v.rational)
		return nil, err
	}
	return v, nil
}

func (s *realValueStore) retain(v *realValue) error {
	if v == nil {
		return nil
	}
	if s == nil || v.store != s || v.deleted || v.refs == 0 || v.refs == ^uint32(0) {
		return arena.ErrInvalidSlot
	}
	v.refs++
	return nil
}

func (s *realValueStore) release(v *realValue) error {
	if v == nil {
		return nil
	}
	if s == nil || v.store != s || v.deleted || v.refs == 0 {
		return arena.ErrInvalidSlot
	}
	if v.refs > 1 {
		v.refs--
		return nil
	}
	if v.isRational {
		if v.rationalInterval != nil {
			if err := v.rationalInterval.Close(); err != nil {
				return err
			}
			v.rationalInterval = nil
		}
		if err := s.numbers.Del(&v.rational); err != nil {
			return err
		}
	} else {
		if v.functionInterval != nil {
			if err := v.functionInterval.Close(); err != nil {
				return err
			}
			v.functionInterval = nil
		}
		if err := s.resetPolynomial(&v.functionNumerator); err != nil {
			return err
		}
		if err := v.root.release(); err != nil {
			return err
		}
		v.root = nil
	}
	v.refs = 0
	v.deleted = true
	return nil
}

// realPolynomial is the concrete ptr_array<value> representation used by
// real-closure polynomial coefficients. Its slice owns references, not values.
type realPolynomial struct {
	coefficients []*realValue
}

// setPolynomial follows reset_p then set_p: release the old coefficients in
// ascending order, install new pointer slots, and retain each new value.
func (s *realValueStore) setPolynomial(dst *realPolynomial, coefficients []*realValue) error {
	if dst == nil || len(coefficients) == 0 || coefficients[len(coefficients)-1] == nil {
		return arena.ErrInvalidSlot
	}
	for _, value := range coefficients {
		if value != nil && (value.store != s || value.deleted || value.refs == 0) {
			return arena.ErrInvalidSlot
		}
	}
	if err := s.resetPolynomial(dst); err != nil {
		return err
	}
	dst.coefficients = append([]*realValue(nil), coefficients...)
	for i, value := range dst.coefficients {
		if err := s.retain(value); err != nil {
			for j := 0; j < i; j++ {
				_ = s.release(dst.coefficients[j])
			}
			dst.coefficients = nil
			return err
		}
	}
	return nil
}

func (s *realValueStore) resetPolynomial(dst *realPolynomial) error {
	if dst == nil {
		return arena.ErrInvalidSlot
	}
	for i, value := range dst.coefficients {
		if err := s.release(value); err != nil {
			return err
		}
		dst.coefficients[i] = nil
	}
	dst.coefficients = nil
	return nil
}
