package realclosure

// Source: src/math/realclosure/realclosure.cpp:4087-4097.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) refineRationalCoefficientIntervals(coefficients []*realValue, bounds *BinaryInterval, precision uint32) error {
	if bounds == nil {
		return arena.ErrInvalidSlot
	}
	for _, coefficient := range coefficients {
		if coefficient == nil {
			continue
		}
		if !s.rationalOperand(coefficient) || !coefficient.isRational {
			return arena.ErrInvalidSlot
		}
		interval, err := s.cachedRationalApproxInterval(coefficient, bounds.numbers)
		if err != nil {
			return err
		}
		if err := s.refineSturmRationalInterval(coefficient, interval, precision); err != nil {
			return err
		}
	}
	return nil
}

// refineCoefficientIntervals follows refine_coeffs_interval: every present
// coefficient is refined through the same value dispatcher used by the C++
// manager, including algebraic and transcendental coefficients.
func (s *realValueStore) refineCoefficientIntervals(coefficients []*realValue, bounds *BinaryInterval, precision uint32) (bool, error) {
	if bounds == nil || bounds.numbers == nil {
		return false, arena.ErrInvalidSlot
	}
	for _, coefficient := range coefficients {
		if coefficient == nil {
			continue
		}
		refined, err := s.refineAnyRetainedInterval(coefficient, bounds.numbers, precision)
		if err != nil || !refined {
			return refined, err
		}
	}
	return true, nil
}
