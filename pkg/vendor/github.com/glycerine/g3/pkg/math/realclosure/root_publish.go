package realclosure

// Source: src/math/realclosure/realclosure.cpp:1780-1860 and 2207-2222.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (o *algebraicRootOwner) publishExactBinaryRoot(bound *numeral.BinaryRational) (*realValue, error) {
	if o == nil || o.values == nil || o.intervals == nil || bound == nil {
		return nil, arena.ErrInvalidSlot
	}
	r := numeral.NewRational()
	if err := o.intervals.ToRational(o.values.numbers, bound, &r); err != nil {
		return nil, errors.Join(err, o.values.releaseSturmRational(&r))
	}
	return o.values.ownNonzeroRational(&r)
}

func (o *algebraicRootOwner) publishSingleIsolatedRoot(p []*realValue, interval, isolating *BinaryInterval) (value *realValue, err error) {
	if o == nil || o.values == nil || interval == nil || isolating == nil {
		return nil, arena.ErrInvalidSlot
	}
	one, err := o.values.newRational(1, 1)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, o.values.releaseSturmScratch(one)) }()
	root, err := o.mkAlgebraic(p, interval, isolating, nil, -1)
	if err != nil {
		return nil, err
	}
	value, err = o.values.newAlgebraicValue(root, one)
	if err != nil {
		return nil, errors.Join(err, root.discardUnpublished())
	}
	return value, nil
}
