package realclosure

// Source: src/math/realclosure/realclosure.cpp:2282-2434.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (o *algebraicRootOwner) isolateNonzeroSquareFreeRationalRoots(p []*realValue, usePrem bool) (roots []*realValue, err error) {
	if o == nil || o.values == nil || o.intervals == nil || len(p) == 0 || p[0] == nil || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, coefficient := range p {
		if !o.values.rationalOperand(coefficient) {
			return nil, arena.ErrInvalidSlot
		}
	}
	if len(p) == 1 {
		return nil, nil
	}
	if len(p) == 2 {
		negativeConstant, err := o.values.negateRational(p[0])
		if err != nil {
			return nil, err
		}
		value, err := o.values.divideRationals(negativeConstant, p[1])
		releaseErr := o.values.releaseSturmScratch(negativeConstant)
		if err != nil {
			return nil, errors.Join(err, releaseErr)
		}
		return []*realValue{value}, releaseErr
	}
	negLower, hasNegLower, err := o.values.negativeRootLowerBound(p, o.intervals)
	if err != nil {
		return nil, err
	}
	negUpper, hasNegUpper, err := o.values.negativeRootUpperBound(p, o.intervals)
	if err != nil {
		return nil, err
	}
	posLower, hasPosLower, err := o.values.positiveRootLowerBound(p, o.intervals)
	if err != nil {
		return nil, err
	}
	posUpper, hasPosUpper, err := o.values.positiveRootUpperBound(p, o.intervals)
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
	negativeCount, err := o.values.rootCountBinaryIntervalRational(p, negativeHalf, usePrem)
	if err != nil {
		return nil, err
	}
	positiveCount, err := o.values.rootCountBinaryIntervalRational(p, positiveHalf, usePrem)
	if err != nil {
		return nil, err
	}
	if negativeCount > 0 {
		part, childErr := o.bisectRationalRoots(p, negative, negativeHalf, negativeCount, usePrem)
		roots = append(roots, part...)
		if childErr != nil {
			return roots, childErr
		}
	}
	if positiveCount > 0 {
		part, childErr := o.bisectRationalRoots(p, positive, positiveHalf, positiveCount, usePrem)
		roots = append(roots, part...)
		if childErr != nil {
			return roots, childErr
		}
	}
	return roots, nil
}

// isolateSquareFreeRationalRoots is the outer source dispatch after the
// square-free producer.  It strips the maximal x^k prefix and appends the
// source's distinguished zero numeral after all negative and positive roots.
func (o *algebraicRootOwner) isolateSquareFreeRationalRoots(p []*realValue, usePrem bool) ([]*realValue, error) {
	if o == nil || o.values == nil || len(p) == 0 || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	first := 0
	for first < len(p) && p[first] == nil {
		first++
	}
	if first == len(p) {
		return nil, arena.ErrInvalidSlot
	}
	roots, err := o.isolateNonzeroSquareFreeRationalRoots(p[first:], usePrem)
	if err != nil {
		return roots, err
	}
	if first > 0 {
		roots = append(roots, nil)
	}
	return roots, nil
}
