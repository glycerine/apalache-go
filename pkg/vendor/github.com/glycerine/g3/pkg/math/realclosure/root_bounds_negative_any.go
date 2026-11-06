package realclosure

// Source: src/math/realclosure/realclosure.cpp:1509-1550, 1559-1564.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) negativeRootLowerBoundAny(p []*realValue, bounds *numeral.BinaryRationalManager, usePrem bool) (int32, bool, error) {
	if len(p) < 2 || p[len(p)-1] == nil || bounds == nil {
		return 0, false, arena.ErrInvalidSlot
	}
	lcSign, err := s.determineAnyRetainedValueSign(p[len(p)-1], usePrem)
	if err != nil {
		return 0, false, err
	}
	if (len(p)-1)&1 == 0 {
		lcSign = -lcSign
	}
	lcMagnitude, ok, err := s.coefficientMagnitudeAny(p[len(p)-1], bounds, true)
	if err != nil || !ok {
		return 0, ok, err
	}
	result := -int32(s.rationalInitialPrecision())
	for k := 2; k <= len(p); k++ {
		index := len(p) - k
		coefficient := p[index]
		if coefficient == nil {
			continue
		}
		sign, err := s.determineAnyRetainedValueSign(coefficient, usePrem)
		if err != nil {
			return 0, false, err
		}
		if index&1 == 0 {
			sign = -sign
		}
		if sign == lcSign {
			continue
		}
		magnitude, ok, err := s.coefficientMagnitudeAny(coefficient, bounds, false)
		if err != nil || !ok {
			return 0, ok, err
		}
		candidate := (magnitude-lcMagnitude)/int32(k) + 2
		if candidate > result {
			result = candidate
		}
	}
	return result, true, nil
}

func (s *realValueStore) negativeRootUpperBoundAny(p []*realValue, bounds *numeral.BinaryRationalManager, usePrem bool) (int32, bool, error) {
	magnitude, ok, err := s.negativeRootLowerBoundAny(reverseRealPolynomial(p), bounds, usePrem)
	return -magnitude, ok, err
}
