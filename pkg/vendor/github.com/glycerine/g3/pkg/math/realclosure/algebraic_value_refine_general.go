package realclosure

// Source: src/math/realclosure/realclosure.cpp:4247-4311.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) refineAlgebraicPolynomialValue(value *realValue, precision uint32) (bool, error) {
	if value == nil || value.isRational || value.root == nil || value.functionInterval == nil {
		return false, arena.ErrInvalidSlot
	}
	for workPrecision := precision; ; workPrecision++ {
		refinedCoefficients, err := s.refineCoefficientIntervals(value.functionNumerator.coefficients, value.root.interval, workPrecision)
		if err != nil || !refinedCoefficients {
			return refinedCoefficients, err
		}
		refined, err := s.refineSingleAlgebraicRoot(value.root, workPrecision)
		if err != nil || !refined {
			return refined, err
		}
		if err = s.updateAlgebraicPolynomialValueInterval(value); err != nil {
			return false, err
		}
		precise, err := binaryIntervalPrecise(value.functionInterval, precision)
		if err != nil || precise {
			return precise, err
		}
		if workPrecision == ^uint32(0) {
			return false, arena.ErrBounds
		}
	}
}
