package realclosure

// Source: src/math/realclosure/realclosure.cpp:5619-5655.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (o *algebraicRootOwner) rootRankedRetainedValue(value *realValue, degree uint32, usePrem bool) (result *realValue, err error) {
	if o == nil || o.values == nil || o.intervals == nil || value != nil && (value.store != o.values || value.deleted || value.refs == 0) {
		return nil, arena.ErrInvalidSlot
	}
	if value != nil && !value.isRational && value.root.owner != o {
		return nil, arena.ErrInvalidSlot
	}
	if degree == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if degree == 1 || value == nil {
		if err := o.values.retain(value); err != nil {
			return nil, err
		}
		return value, nil
	}
	sign, err := o.values.determineRankedRetainedValueSign(value, usePrem)
	if err != nil {
		return nil, err
	}
	if sign < 0 && degree&1 == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if sign < 0 {
		positive, e := o.values.negateRankedRetainedValue(value, usePrem)
		if e != nil {
			return nil, e
		}
		positiveRoot, e := o.rootRankedRetainedValue(positive, degree, usePrem)
		releaseErr := o.values.releaseSturmScratch(positive)
		if e != nil {
			return nil, errors.Join(e, releaseErr)
		}
		result, e = o.values.negateRankedRetainedValue(positiveRoot, usePrem)
		return result, errors.Join(e, releaseErr, o.values.releaseSturmScratch(positiveRoot))
	}
	negative, err := o.values.negateRankedRetainedValue(value, usePrem)
	if err != nil {
		return nil, err
	}
	one, err := o.values.newRational(1, 1)
	if err != nil {
		return nil, errors.Join(err, o.values.releaseSturmScratch(negative))
	}
	polynomial := make([]*realValue, int(degree)+1)
	polynomial[0], polynomial[degree] = negative, one
	defer func() {
		err = errors.Join(err, o.values.releaseSturmScratch(negative), o.values.releaseSturmScratch(one))
	}()
	roots, err := o.isolateAnyRoots(polynomial, usePrem)
	if err != nil {
		return nil, err
	}
	return o.values.selectRankedPrincipalRoot(roots, usePrem)
}
