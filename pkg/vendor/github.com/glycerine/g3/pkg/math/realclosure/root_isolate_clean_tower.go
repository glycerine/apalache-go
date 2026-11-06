package realclosure

// Source: src/math/realclosure/realclosure.cpp:2376-2415.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (o *algebraicRootOwner) isolateNonzeroCleanTowerRoots(p []*realValue, usePrem bool) (roots []*realValue, err error) {
	clean, err := o.values.hasCleanPolynomialDenominators(p)
	if err != nil || !clean {
		if err == nil {
			err = arena.ErrInvalidSlot
		}
		return nil, err
	}
	squareFree, err := o.values.squareFreeTowerPolynomial(p, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, o.values.releaseSturmCoefficients(squareFree)) }()
	return o.isolateNonzeroSquareFreeTowerRoots(squareFree, usePrem)
}
