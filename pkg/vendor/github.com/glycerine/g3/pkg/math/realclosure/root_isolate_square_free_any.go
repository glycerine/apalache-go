package realclosure

// Source: src/math/realclosure/realclosure.cpp:2352-2374.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (o *algebraicRootOwner) isolateNonzeroSquareFreeAnyRoots(p []*realValue, usePrem bool) ([]*realValue, error) {
	if o == nil || o.values == nil || len(p) == 0 || p[0] == nil || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	if len(p) == 1 {
		return nil, nil
	}
	if len(p) == 2 {
		negative, err := o.values.negateRetainedValue(p[0])
		if err != nil {
			return nil, err
		}
		value, divideErr := o.values.divideRetainedValues(negative, p[1])
		releaseErr := o.values.releaseSturmScratch(negative)
		if divideErr != nil {
			return nil, errors.Join(divideErr, releaseErr)
		}
		return []*realValue{value}, releaseErr
	}
	return o.isolateNonlinearSquareFreeAnyRoots(p, usePrem)
}
