package realclosure

// Source: src/math/realclosure/realclosure.cpp:2196-2280.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (o *algebraicRootOwner) bisectTowerRoots(p []*realValue, interval, isolating *BinaryInterval, count int, usePrem bool) (roots []*realValue, err error) {
	if o == nil || o.values == nil || interval == nil || isolating == nil || count < 0 || interval.numbers != o.intervals || isolating.numbers != o.intervals {
		return nil, arena.ErrInvalidSlot
	}
	if count == 0 {
		return nil, nil
	}
	if interval.lowerInf || interval.upperInf {
		if count != 1 {
			return nil, arena.ErrInvalidSlot
		}
		value, err := o.publishSingleIsolatedRoot(p, interval, isolating)
		return []*realValue{value}, err
	}
	if count == 1 {
		upperSign, err := o.values.signRankedPolynomialAtBinaryBound(p, o.intervals, &interval.upper, usePrem)
		if err != nil {
			return nil, err
		}
		if upperSign == 0 {
			value, err := o.publishExactBinaryRoot(&interval.upper)
			return []*realValue{value}, err
		}
		value, err := o.publishSingleIsolatedRoot(p, interval, isolating)
		return []*realValue{value}, err
	}
	midpoint := numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, o.intervals.Del(&midpoint)) }()
	if err = o.intervals.Add(&interval.lower, &interval.upper, &midpoint); err != nil {
		return nil, err
	}
	if err = o.intervals.Div2(&midpoint); err != nil {
		return nil, err
	}
	left, err := NewBinaryInterval(o.intervals)
	if err != nil {
		return nil, err
	}
	right, err := NewBinaryInterval(o.intervals)
	if err != nil {
		return nil, errors.Join(err, left.Close())
	}
	defer func() { err = errors.Join(err, left.Close(), right.Close()) }()
	if err = left.SetFinite(&interval.lower, &midpoint, interval.lowerOpen, false); err != nil {
		return nil, err
	}
	if err = right.SetFinite(&midpoint, &interval.upper, true, interval.upperOpen); err != nil {
		return nil, err
	}
	leftCount, err := o.values.rootCountBinaryIntervalRanked(p, left, usePrem)
	if err != nil {
		return nil, fmt.Errorf("tower bisection left count: %w", err)
	}
	if leftCount < 0 || leftCount > count {
		return nil, fmt.Errorf("tower bisection left count %d of %d: %w", leftCount, count, arena.ErrInvalidSlot)
	}
	rightCount := count - leftCount
	if leftCount > 0 {
		leftIsolation := isolating
		if rightCount > 0 {
			leftIsolation = left
		}
		part, childErr := o.bisectTowerRoots(p, left, leftIsolation, leftCount, usePrem)
		roots = append(roots, part...)
		if childErr != nil {
			return roots, childErr
		}
	}
	if rightCount > 0 {
		rightIsolation := isolating
		if leftCount > 0 {
			rightIsolation = right
		}
		part, childErr := o.bisectTowerRoots(p, right, rightIsolation, rightCount, usePrem)
		roots = append(roots, part...)
		if childErr != nil {
			return roots, childErr
		}
	}
	return roots, nil
}
