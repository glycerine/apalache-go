package realclosure

// Source: src/math/realclosure/realclosure.cpp:2282-2350.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
)

func (o *algebraicRootOwner) isolateNonlinearSquareFreeTowerRoots(p []*realValue, usePrem bool) (roots []*realValue, err error) {
	if o == nil || o.values == nil || len(p) <= 2 || p[0] == nil || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	negLower, hasNegLower, err := o.values.negativeRootLowerBoundTower(p, o.intervals, usePrem)
	if err != nil {
		return nil, err
	}
	negUpper, hasNegUpper, err := o.values.negativeRootUpperBoundTower(p, o.intervals, usePrem)
	if err != nil {
		return nil, err
	}
	posLower, hasPosLower, err := o.values.positiveRootLowerBoundTower(p, o.intervals, usePrem)
	if err != nil {
		return nil, err
	}
	posUpper, hasPosUpper, err := o.values.positiveRootUpperBoundTower(p, o.intervals, usePrem)
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
	negativeCount, err := o.values.rootCountHalfLineTower(p, false, usePrem)
	if err != nil {
		return nil, fmt.Errorf("tower negative half count: %w", err)
	}
	positiveCount, err := o.values.rootCountHalfLineTower(p, true, usePrem)
	if err != nil {
		return nil, fmt.Errorf("tower positive half count: %w", err)
	}
	if negativeCount > 0 {
		part, childErr := o.bisectTowerRoots(p, negative, negativeHalf, negativeCount, usePrem)
		roots = append(roots, part...)
		if childErr != nil {
			return roots, fmt.Errorf("tower negative isolation: %w", childErr)
		}
	}
	if positiveCount > 0 {
		part, childErr := o.bisectTowerRoots(p, positive, positiveHalf, positiveCount, usePrem)
		roots = append(roots, part...)
		if childErr != nil {
			return roots, fmt.Errorf("tower positive isolation: %w", childErr)
		}
	}
	return roots, nil
}
