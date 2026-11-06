package realclosure

// Source: src/math/realclosure/realclosure.cpp:1588-1611,1780-1860,
// and 1943-2120.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// isolateRationalRootsBySigns implements the retained rational-coefficient
// sign_det_isolate_roots path. Returned values own the published algebraic
// roots. When publication stops, an already-published prefix remains returned
// and owned by the caller, matching the source output vector prefix.
func (o *algebraicRootOwner) isolateRationalRootsBySigns(p []*realValue, numRoots int, enclosure, isolating *BinaryInterval, usePrem bool) (roots []*realValue, err error) {
	if o == nil || o.values == nil || o.intervals == nil || enclosure == nil || isolating == nil || enclosure.numbers != o.intervals || isolating.numbers != o.intervals || numRoots < 2 || len(p) < 3 || p[0] == nil || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, value := range p {
		if !o.values.rationalOperand(value) {
			return nil, arena.ErrInvalidSlot
		}
	}
	if err = o.values.sequenceReady(); err != nil {
		return nil, err
	}
	one, err := o.values.newRational(1, 1)
	if err != nil {
		return nil, err
	}
	derivatives, err := o.values.derivativeSequenceRational(p)
	if err != nil {
		return nil, errors.Join(err, o.values.releaseSturmScratch(one))
	}
	state := newSignDetermination(numRoots)
	var result *signDet
	defer func() {
		if state != nil && state.store != nil {
			err = errors.Join(err, state.closePolynomials())
		}
		if result != nil && result.refs == 0 && !result.deleted {
			err = errors.Join(err, result.dispose())
		}
		err = errors.Join(err, o.values.closePolynomialSequenceAfterTemporaries(derivatives))
		err = errors.Join(err, o.values.releaseSturmScratch(one))
	}()
	if err = state.attachInitialPolynomial(o.values, one); err != nil {
		return nil, err
	}
	allOne := false
	for i := len(derivatives); i > 0; {
		i--
		if err = o.values.checkpoint(); err != nil {
			return roots, err
		}
		_, allOne, err = state.advanceBoundedRational(p, derivatives[i].coefficients, isolating, usePrem)
		if err != nil {
			return roots, err
		}
		if allOne {
			break
		}
	}
	if !allOne || state.matrix.rows != numRoots || state.matrix.cols != numRoots {
		return roots, arena.ErrInvalidSlot
	}
	result, err = state.finish()
	if err != nil {
		return roots, err
	}
	for index := 0; index < numRoots; index++ {
		root, publishErr := o.mkAlgebraic(p, enclosure, isolating, result, index)
		if publishErr != nil {
			return roots, publishErr
		}
		value, publishErr := o.values.newAlgebraicValue(root, one)
		if publishErr != nil {
			return roots, errors.Join(publishErr, root.discardUnpublished())
		}
		roots = append(roots, value)
	}
	return roots, nil
}
