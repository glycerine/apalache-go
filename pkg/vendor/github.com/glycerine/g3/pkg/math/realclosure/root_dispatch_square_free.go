package realclosure

// Source: src/math/realclosure/realclosure.cpp:2381-2434. This completes the
// rational retained outer dispatch by inserting the source square-free step
// before the Batch 2 nonzero/square-free isolator.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (o *algebraicRootOwner) isolateRationalRoots(p []*realValue, usePrem bool) (roots []*realValue, err error) {
	if o == nil || o.values == nil || len(p) == 0 || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	if len(p) == 1 {
		if err := o.values.checkRationalPolynomial(p, false); err != nil {
			return nil, err
		}
		return nil, nil
	}
	first := 0
	for first < len(p) && p[first] == nil {
		first++
	}
	if first == len(p) {
		return nil, arena.ErrInvalidSlot
	}
	squareFree, err := o.values.squareFreeRationalPolynomial(p[first:], usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, o.values.releaseSturmCoefficients(squareFree)) }()
	roots, err = o.isolateNonzeroSquareFreeRationalRoots(squareFree, usePrem)
	if err != nil {
		return roots, err
	}
	if first > 0 {
		roots = append(roots, nil)
	}
	return roots, nil
}
