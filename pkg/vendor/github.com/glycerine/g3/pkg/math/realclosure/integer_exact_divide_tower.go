package realclosure

// Source: src/math/realclosure/realclosure.cpp:3591-3631.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) exactDivideTowerValueByInteger(value *realValue, divisor *numeral.Int, usePrem bool) (result *realValue, err error) {
	if value == nil {
		return nil, nil
	}
	scalar, err := s.ownPositiveIntegerScalar(divisor)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(scalar)) }()
	return s.divideRankedRetainedValues(value, scalar, usePrem)
}
