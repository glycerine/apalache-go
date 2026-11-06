package realclosure

// Source: src/math/realclosure/realclosure.cpp:6312-6316.

func (s *realValueStore) swapNumeralSlots(left, right **realValue) error {
	if err := s.validateNumeralSlot(left); err != nil {
		return err
	}
	if err := s.validateNumeralSlot(right); err != nil {
		return err
	}
	*left, *right = *right, *left
	return nil
}
