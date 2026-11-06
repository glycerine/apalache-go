package realclosure

// Source: src/math/realclosure/realclosure.cpp:5990-6011 manager interval
// display/query preparation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) snapshotAnyRetainedInterval(value *realValue, bounds *numeral.BinaryRationalManager, precision uint32) (snapshot *BinaryInterval, err error) {
	if value == nil {
		return s.cloneValueInterval(nil, bounds, precision)
	}
	if _, err = s.refineAnyRetainedInterval(value, bounds, precision); err != nil {
		return nil, err
	}
	interval, err := s.valueInterval(value, bounds)
	if err != nil {
		return nil, err
	}
	snapshot, err = NewBinaryInterval(bounds)
	if err != nil {
		return nil, err
	}
	if err = snapshot.SetFrom(interval); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	return snapshot, nil
}
