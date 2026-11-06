package realclosure

// Source: src/math/realclosure/realclosure.cpp:2196-2280.
//
// The source carries a precomputed Sturm sequence down the recursion.  This
// concrete retained instantiation asks the existing rational Sturm producer
// for each half; it preserves the source's half-open (a,b] ownership and
// publication decisions while avoiding a second sequence ownership protocol.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (o *algebraicRootOwner) bisectRationalRoots(p []*realValue, interval, isolating *BinaryInterval, count int, usePrem bool) (roots []*realValue, err error) {
	if o == nil || o.values == nil || interval == nil || isolating == nil || count < 0 || interval.numbers != o.intervals || isolating.numbers != o.intervals {
		return nil, arena.ErrInvalidSlot
	}
	if count == 0 {
		return nil, nil
	}
	if interval.lowerInf || interval.upperInf {
		if count < 2 {
			value, err := o.publishSingleIsolatedRoot(p, interval, isolating)
			return []*realValue{value}, err
		}
		return o.isolateRationalRootsBySigns(p, count, interval, isolating, usePrem)
	}
	if count == 1 {
		upperSign, err := o.values.signAtBinaryBound(p, o.intervals, &interval.upper)
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
	mid := numeral.NewBinaryRational()
	defer func() { err = errors.Join(err, o.intervals.Del(&mid)) }()
	if err = o.intervals.Add(&interval.lower, &interval.upper, &mid); err != nil {
		return nil, err
	}
	if err = o.intervals.Div2(&mid); err != nil {
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
	if err = left.SetFinite(&interval.lower, &mid, interval.lowerOpen, false); err != nil {
		return nil, err
	}
	if err = right.SetFinite(&mid, &interval.upper, true, interval.upperOpen); err != nil {
		return nil, err
	}
	leftCount, err := o.values.rootCountBinaryIntervalRational(p, left, usePrem)
	if err != nil {
		return nil, err
	}
	if leftCount < 0 || leftCount > count {
		return nil, arena.ErrInvalidSlot
	}
	rightCount := count - leftCount
	if leftCount > 0 {
		leftIsolation := isolating
		if rightCount > 0 {
			leftIsolation = left
		}
		leftRoots, childErr := o.bisectRationalRoots(p, left, leftIsolation, leftCount, usePrem)
		roots = append(roots, leftRoots...)
		if childErr != nil {
			return roots, childErr
		}
	}
	if rightCount > 0 {
		rightIsolation := isolating
		if leftCount > 0 {
			rightIsolation = right
		}
		rightRoots, childErr := o.bisectRationalRoots(p, right, rightIsolation, rightCount, usePrem)
		roots = append(roots, rightRoots...)
		if childErr != nil {
			return roots, childErr
		}
	}
	return roots, nil
}
