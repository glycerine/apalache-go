package realclosure

// Source: src/math/realclosure/realclosure.cpp:2282-2350.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (o *algebraicRootOwner) isolateNonlinearSquareFreeAnyRoots(p []*realValue, usePrem bool) (roots []*realValue, err error) {
	if o == nil || o.values == nil || len(p) <= 2 || p[0] == nil || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	negLower, hasNegLower, err := o.values.negativeRootLowerBoundAny(p, o.intervals, usePrem)
	if err != nil {
		return nil, err
	}
	negUpper, hasNegUpper, err := o.values.negativeRootUpperBoundAny(p, o.intervals, usePrem)
	if err != nil {
		return nil, err
	}
	posLower, hasPosLower, err := o.values.positiveRootLowerBoundAny(p, o.intervals, usePrem)
	if err != nil {
		return nil, err
	}
	posUpper, hasPosUpper, err := o.values.positiveRootUpperBoundAny(p, o.intervals, usePrem)
	if err != nil {
		return nil, err
	}
	negative, err := makeRootBoundInterval(o.intervals, negLower, hasNegLower, negUpper, hasNegUpper, true)
	if err != nil {
		return nil, err
	}
	positive, err := makeRootBoundInterval(o.intervals, posLower, hasPosLower, posUpper, hasPosUpper, false)
	if err != nil {
		return nil, errors.Join(err, negative.Close())
	}
	negativeHalf, err := makeRootBoundInterval(o.intervals, 0, false, 0, false, true)
	if err != nil {
		return nil, errors.Join(err, negative.Close(), positive.Close())
	}
	positiveHalf, err := makeRootBoundInterval(o.intervals, 0, false, 0, false, false)
	if err != nil {
		return nil, errors.Join(err, negative.Close(), positive.Close(), negativeHalf.Close())
	}
	defer func() {
		err = errors.Join(err, negative.Close(), positive.Close(), negativeHalf.Close(), positiveHalf.Close())
	}()
	negativeCount, err := o.values.rootCountBinaryIntervalAny(p, negativeHalf, usePrem)
	if err != nil {
		return nil, err
	}
	positiveCount, err := o.values.rootCountBinaryIntervalAny(p, positiveHalf, usePrem)
	if err != nil {
		return nil, err
	}
	if negativeCount > 0 {
		var part []*realValue
		var childErr error
		if negativeCount > 1 && !hasNegLower {
			part, childErr = o.isolateAnyRootsBySigns(p, negativeCount, negativeHalf, negativeHalf, usePrem)
		} else {
			part, childErr = o.bisectAnyRoots(p, negative, negativeHalf, negativeCount, usePrem)
		}
		roots = append(roots, part...)
		if childErr != nil {
			return roots, childErr
		}
	}
	if positiveCount > 0 {
		var part []*realValue
		var childErr error
		if positiveCount > 1 && !hasPosUpper {
			part, childErr = o.isolateAnyRootsBySigns(p, positiveCount, positiveHalf, positiveHalf, usePrem)
		} else {
			part, childErr = o.bisectAnyRoots(p, positive, positiveHalf, positiveCount, usePrem)
		}
		roots = append(roots, part...)
		if childErr != nil {
			return roots, childErr
		}
	}
	return roots, nil
}
