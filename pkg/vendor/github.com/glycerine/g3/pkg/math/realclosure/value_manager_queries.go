package realclosure

// Source: src/math/realclosure/realclosure.cpp:2449-2495 and manager wrappers
// at 6008-6045.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) isZeroValue(value *realValue) (bool, error) {
	sign, err := s.signValue(value)
	return sign == 0, err
}

func (s *realValueStore) isPositiveValue(value *realValue) (bool, error) {
	sign, err := s.signValue(value)
	return sign > 0, err
}

func (s *realValueStore) isNegativeValue(value *realValue) (bool, error) {
	sign, err := s.signValue(value)
	return sign < 0, err
}

func (s *realValueStore) isIntegerValue(value *realValue) (bool, error) {
	if value == nil {
		return true, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 {
		return false, arena.ErrInvalidSlot
	}
	if !value.isRational {
		// Source is_algebraic_int is deliberately still TODO and returns false.
		return false, nil
	}
	return s.numbers.IsInt(&value.rational)
}

func (s *realValueStore) dependsOnInfinitesimalsValue(value *realValue) (bool, error) {
	if value == nil {
		return false, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 {
		return false, arena.ErrInvalidSlot
	}
	if value.isRational {
		return false, nil
	}
	if value.root == nil || value.root.deleted {
		return false, arena.ErrInvalidSlot
	}
	if value.root.infinitesimals || value.root.dependsOnInfinitesimals {
		return true, nil
	}
	for _, polynomial := range [][]*realValue{value.functionNumerator.coefficients, value.functionDenominator.coefficients} {
		for _, coefficient := range polynomial {
			depends, err := s.dependsOnInfinitesimalsValue(coefficient)
			if err != nil || depends {
				return depends, err
			}
		}
	}
	return false, nil
}
