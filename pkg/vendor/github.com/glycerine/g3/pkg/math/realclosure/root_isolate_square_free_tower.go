package realclosure

// Source: src/math/realclosure/realclosure.cpp:2352-2374.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (o *algebraicRootOwner) isolateNonzeroSquareFreeTowerRoots(p []*realValue, usePrem bool) ([]*realValue, error) {
	if o == nil || o.values == nil || len(p) == 0 || p[0] == nil || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	if len(p) == 1 {
		return nil, nil
	}
	if len(p) == 2 {
		negative, err := o.values.negateRankedRetainedValue(p[0], usePrem)
		if err != nil {
			return nil, err
		}
		value, divideErr := o.values.divideRankedRetainedValues(negative, p[1], usePrem)
		releaseErr := o.values.releaseSturmScratch(negative)
		if divideErr != nil {
			return nil, errors.Join(divideErr, releaseErr)
		}
		return []*realValue{value}, releaseErr
	}
	return o.isolateNonlinearSquareFreeTowerRoots(p, usePrem)
}
