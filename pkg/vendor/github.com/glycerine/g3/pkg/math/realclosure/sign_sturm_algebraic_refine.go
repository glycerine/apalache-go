package realclosure

// Source: src/math/realclosure/realclosure.cpp:4247-4311.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// refineSturmAlgebraicInterval bisects a single-root algebraic extension and
// updates the retained numerator-x rational-function interval. Defining
// polynomials with nonrational coefficients and sign-determined multi-root
// extensions still require the source's broader value operations.
func (s *realValueStore) refineSturmAlgebraicInterval(value *realValue, precision uint32) (refined bool, err error) {
	if s == nil || value == nil || value.store != s || value.deleted || value.refs == 0 || value.root == nil || value.functionInterval == nil {
		return false, arena.ErrInvalidSlot
	}
	root := value.root
	if root.deleted || root.owner == nil || root.owner.values != s || root.interval == nil || root.owner.intervals != value.functionInterval.numbers {
		return false, arena.ErrInvalidSlot
	}
	if root.signs != nil || root.interval.lowerInf || root.interval.upperInf {
		return false, nil
	}
	coefficients := root.polynomial.coefficients
	if len(coefficients) == 0 || coefficients[len(coefficients)-1] == nil {
		return false, arena.ErrInvalidSlot
	}
	for _, coefficient := range coefficients {
		if !s.rationalOperand(coefficient) {
			return false, nil
		}
	}
	bounds := root.owner.intervals
	mid := numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, s.releaseSturmBinary(bounds, &mid)) }()
	var lowerSign int32
	for {
		width := numeral.NewBinaryRational()
		e := bounds.Sub(&root.interval.upper, &root.interval.lower, &width)
		if e != nil {
			return false, errors.Join(e, s.releaseSturmBinary(bounds, &width))
		}
		precise, e := bounds.LessOneDiv2k(&width, precision)
		e = errors.Join(e, s.releaseSturmBinary(bounds, &width))
		if e != nil {
			return false, e
		}
		if precise {
			return true, value.functionInterval.SetFrom(root.interval)
		}
		equal, e := bounds.Equal(&root.interval.lower, &root.interval.upper)
		if e != nil || equal {
			return false, e
		}
		if e = bounds.Add(&root.interval.lower, &root.interval.upper, &mid); e != nil {
			return false, e
		}
		if e = bounds.Div2(&mid); e != nil {
			return false, e
		}
		midSign, e := s.signAtBinaryBound(coefficients, bounds, &mid)
		if e != nil {
			return false, e
		}
		if midSign == 0 {
			if e = root.interval.SetFinite(&mid, &mid, false, false); e != nil {
				return false, e
			}
			return true, value.functionInterval.SetFrom(root.interval)
		}
		if lowerSign == 0 {
			lowerSign, e = s.signAtBinaryBound(coefficients, bounds, &root.interval.lower)
			if e != nil {
				return false, e
			}
			if lowerSign == 0 {
				return false, arena.ErrInvalidSlot
			}
		}
		if midSign == lowerSign {
			e = root.interval.SetFinite(&mid, &root.interval.upper, true, root.interval.upperOpen)
		} else {
			e = root.interval.SetFinite(&root.interval.lower, &mid, root.interval.lowerOpen, true)
		}
		if e != nil {
			return false, e
		}
	}
}
