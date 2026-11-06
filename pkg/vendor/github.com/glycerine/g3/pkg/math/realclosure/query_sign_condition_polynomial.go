package realclosure

// Source: src/math/realclosure/realclosure.cpp:3465-3503.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) numSignConditionCoefficients(value *realValue, index int) (int, error) {
	condition, signs, err := s.signConditionAt(value, index)
	if err != nil || condition == nil {
		return 0, err
	}
	if condition.polynomial < 0 || condition.polynomial >= len(signs.qs) {
		return 0, arena.ErrInvalidSlot
	}
	return len(signs.qs[condition.polynomial].coefficients), nil
}

func (s *realValueStore) signConditionCoefficient(value *realValue, conditionIndex, coefficientIndex int) (*realValue, error) {
	condition, signs, err := s.signConditionAt(value, conditionIndex)
	if err != nil || condition == nil || coefficientIndex < 0 {
		return nil, err
	}
	if condition.polynomial < 0 || condition.polynomial >= len(signs.qs) {
		return nil, arena.ErrInvalidSlot
	}
	coefficients := signs.qs[condition.polynomial].coefficients
	if coefficientIndex >= len(coefficients) {
		return nil, nil
	}
	coefficient := coefficients[coefficientIndex]
	if err = s.retain(coefficient); err != nil {
		return nil, err
	}
	return coefficient, nil
}
