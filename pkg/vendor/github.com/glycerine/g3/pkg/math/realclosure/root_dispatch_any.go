package realclosure

// Source: src/math/realclosure/realclosure.cpp:2417-2434.

import "github.com/glycerine/g3/pkg/arena"

func (o *algebraicRootOwner) isolateAnyRoots(p []*realValue, usePrem bool) ([]*realValue, error) {
	if o == nil || o.values == nil || len(p) == 0 || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err := o.values.checkAnyPolynomial(p, false); err != nil {
		return nil, err
	}
	// The source returns before denominator cleaning for a nonzero constant.
	if len(p) == 1 {
		return nil, nil
	}
	first := 0
	for first < len(p) && p[first] == nil {
		first++
	}
	if first == len(p) {
		return nil, arena.ErrInvalidSlot
	}
	roots, err := o.isolateNonzeroAnyRoots(p[first:], usePrem)
	if err != nil {
		return roots, err
	}
	if first > 0 {
		roots = append(roots, nil)
	}
	return roots, nil
}
