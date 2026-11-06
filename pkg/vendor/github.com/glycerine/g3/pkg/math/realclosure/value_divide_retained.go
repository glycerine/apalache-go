package realclosure

// Source: src/math/realclosure/realclosure.cpp:5319-5330 and 5587-5618.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) divideRetainedValues(numerator, denominator *realValue) (result *realValue, err error) {
	if denominator == nil {
		return nil, arena.ErrInvalidSlot
	}
	if numerator == nil {
		return nil, nil
	}
	inverse, err := s.inverseRetainedValue(denominator)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(inverse)) }()
	return s.multiplyRetainedValues(numerator, inverse)
}
