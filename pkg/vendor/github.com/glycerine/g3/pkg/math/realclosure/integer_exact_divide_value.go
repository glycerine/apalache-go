package realclosure

// Source: src/math/realclosure/realclosure.cpp:3390-3458.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) exactDivideValueByInteger(value *realValue, divisor *numeral.Int, usePrem bool) (result *realValue, err error) {
	if value == nil {
		return nil, nil
	}
	scalar, err := s.ownPositiveIntegerScalar(divisor)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(scalar)) }()
	result, err = s.divideRetainedValues(value, scalar)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeAnyRetainedResult(result, usePrem)
}
