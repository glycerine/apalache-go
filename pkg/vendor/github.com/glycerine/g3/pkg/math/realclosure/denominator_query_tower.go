package realclosure

// Source: src/math/realclosure/realclosure.cpp:3136-3162.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) hasCleanTowerDenominator(value *realValue) (bool, error) {
	if value == nil {
		return true, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 {
		return false, arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.numbers.IsInt(&value.rational)
	}
	if value.root == nil || value.root.deleted {
		return false, arena.ErrInvalidSlot
	}
	if value.root.kind != realExtensionAlgebraic && len(value.functionDenominator.coefficients) != 0 && !s.rankedDenominatorIsOne(value.functionDenominator.coefficients) {
		return false, nil
	}
	for _, coefficient := range value.functionNumerator.coefficients {
		clean, err := s.hasCleanTowerDenominator(coefficient)
		if err != nil || !clean {
			return clean, err
		}
	}
	return true, nil
}

func (s *realValueStore) hasCleanTowerPolynomialDenominators(p []*realValue) (bool, error) {
	for _, coefficient := range p {
		clean, err := s.hasCleanTowerDenominator(coefficient)
		if err != nil || !clean {
			return clean, err
		}
	}
	return true, nil
}
