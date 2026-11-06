package realclosure

// Source: src/math/realclosure/realclosure.cpp:5319-5330, 5587-5618.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) divideRankedRetainedValues(numerator, denominator *realValue, usePrem bool) (result *realValue, err error) {
	if denominator == nil {
		return nil, arena.ErrInvalidSlot
	}
	if numerator == nil {
		return nil, nil
	}
	inverse, err := s.inverseRankedRetainedValue(denominator, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(inverse)) }()
	return s.multiplyRankedRetainedValues(numerator, inverse, usePrem)
}
