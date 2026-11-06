package realclosure

// Source: src/math/realclosure/realclosure.cpp:3380-3397.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) numDefiningCoefficients(value *realValue) (int, error) {
	if value == nil || value.isRational {
		return 0, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 || value.root == nil {
		return 0, arena.ErrInvalidSlot
	}
	if _, err := value.root.registryIndex(); err != nil {
		return 0, err
	}
	return len(value.root.polynomial.coefficients), nil
}

func (s *realValueStore) definingCoefficient(value *realValue, index int) (*realValue, error) {
	count, err := s.numDefiningCoefficients(value)
	if err != nil || index < 0 || index >= count {
		return nil, err
	}
	coefficient := value.root.polynomial.coefficients[index]
	if err = s.retain(coefficient); err != nil {
		return nil, err
	}
	return coefficient, nil
}
