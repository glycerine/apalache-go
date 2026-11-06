package realclosure

// Source: src/math/realclosure/realclosure.cpp:4194-4245.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) refineSingleAlgebraicRoot(root *algebraicRoot, precision uint32) (refined bool, err error) {
	if s == nil || root == nil || root.deleted || root.owner == nil || root.owner.values != s || root.interval == nil {
		return false, arena.ErrInvalidSlot
	}
	if err = s.saveExtensionIntervalIfTooSmall(root, precision); err != nil {
		return false, err
	}
	if root.signs != nil || root.interval.lowerInf || root.interval.upperInf {
		return false, nil
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
		if e != nil || precise {
			return precise, e
		}
		if err = s.checkpoint(); err != nil {
			return false, err
		}
		if e = bounds.Add(&root.interval.lower, &root.interval.upper, &mid); e != nil {
			return false, e
		}
		if e = bounds.Div2(&mid); e != nil {
			return false, e
		}
		midSign, e := s.signAtBinaryBound(root.polynomial.coefficients, bounds, &mid)
		if e != nil {
			return false, e
		}
		if midSign == 0 {
			return true, root.interval.SetFinite(&mid, &mid, false, false)
		}
		if lowerSign == 0 {
			lowerSign, e = s.signAtBinaryBound(root.polynomial.coefficients, bounds, &root.interval.lower)
			if e != nil || lowerSign == 0 {
				return false, errors.Join(e, arena.ErrInvalidSlot)
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
