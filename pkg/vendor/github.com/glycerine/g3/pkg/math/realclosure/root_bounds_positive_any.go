package realclosure

// Source: src/math/realclosure/realclosure.cpp:1458-1507, 1552-1557.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) positiveRootUpperBoundAny(p []*realValue, bounds *numeral.BinaryRationalManager, usePrem bool) (int32, bool, error) {
	if len(p) < 2 || p[len(p)-1] == nil || bounds == nil {
		return 0, false, arena.ErrInvalidSlot
	}
	lcSign, err := s.determineAnyRetainedValueSign(p[len(p)-1], usePrem)
	if err != nil {
		return 0, false, err
	}
	lcMagnitude, ok, err := s.coefficientMagnitudeAny(p[len(p)-1], bounds, true)
	if err != nil || !ok {
		return 0, ok, err
	}
	result := -int32(s.rationalInitialPrecision())
	for k := 2; k <= len(p); k++ {
		coefficient := p[len(p)-k]
		if coefficient == nil {
			continue
		}
		sign, err := s.determineAnyRetainedValueSign(coefficient, usePrem)
		if err != nil {
			return 0, false, err
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

func (s *realValueStore) positiveRootLowerBoundAny(p []*realValue, bounds *numeral.BinaryRationalManager, usePrem bool) (int32, bool, error) {
	magnitude, ok, err := s.positiveRootUpperBoundAny(reverseRealPolynomial(p), bounds, usePrem)
	return -magnitude, ok, err
}
