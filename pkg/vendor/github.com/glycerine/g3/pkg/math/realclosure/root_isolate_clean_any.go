package realclosure

// Source: src/math/realclosure/realclosure.cpp:2376-2396.

import "errors"

func (o *algebraicRootOwner) isolateNonzeroCleanAnyRoots(p []*realValue, usePrem bool) (roots []*realValue, err error) {
	squareFree, err := o.values.squareFreeAnyPolynomial(p, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, o.values.releaseSturmCoefficients(squareFree)) }()
	return o.isolateNonzeroSquareFreeAnyRoots(squareFree, usePrem)
}
