package realclosure

// Source: src/math/realclosure/realclosure.cpp:5658-5682, 6128-6165.

import "errors"

// assignOwnedNumeralSlot consumes the reference returned by an arithmetic
// operation after installing a retained reference in destination. Computing
// the result before replacing destination preserves the native manager's
// support for output/input aliasing.
func (s *realValueStore) assignOwnedNumeralSlot(destination **realValue, value *realValue) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return errors.Join(err, s.releaseSturmScratch(value))
	}
	err := s.copyNumeralSlot(destination, value)
	return errors.Join(err, s.releaseSturmScratch(value))
}

func (s *realValueStore) addNumeralSlots(destination **realValue, left, right *realValue, usePrem bool) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	result, err := s.addRankedRetainedValues(left, right, usePrem)
	if err != nil {
		return err
	}
	return s.assignOwnedNumeralSlot(destination, result)
}

func (s *realValueStore) subtractNumeralSlots(destination **realValue, left, right *realValue, usePrem bool) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	result, err := s.subtractRankedRetainedValues(left, right, usePrem)
	if err != nil {
		return err
	}
	return s.assignOwnedNumeralSlot(destination, result)
}

func (s *realValueStore) multiplyNumeralSlots(destination **realValue, left, right *realValue, usePrem bool) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	result, err := s.multiplyRankedRetainedValues(left, right, usePrem)
	if err != nil {
		return err
	}
	return s.assignOwnedNumeralSlot(destination, result)
}

func (s *realValueStore) negateNumeralSlot(destination **realValue, value *realValue, usePrem bool) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	if _, err := s.retainedValueRank(value); err != nil {
		return err
	}
	result, err := s.negateRankedRetainedValue(value, usePrem)
	if err != nil {
		return err
	}
	return s.assignOwnedNumeralSlot(destination, result)
}

func (s *realValueStore) inverseNumeralSlot(destination **realValue, value *realValue, usePrem bool) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	result, err := s.inverseRankedRetainedValue(value, usePrem)
	if err != nil {
		return err
	}
	return s.assignOwnedNumeralSlot(destination, result)
}

func (s *realValueStore) divideNumeralSlots(destination **realValue, numerator, denominator *realValue, usePrem bool) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	result, err := s.divideRankedRetainedValues(numerator, denominator, usePrem)
	if err != nil {
		return err
	}
	return s.assignOwnedNumeralSlot(destination, result)
}

func (s *realValueStore) powerNumeralSlot(destination **realValue, value *realValue, exponent uint32, usePrem bool) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	// Exponent zero can be answered without inspecting the operand, but a
	// manager numeral must still belong to this manager.
	if _, err := s.retainedValueRank(value); err != nil {
		return err
	}
	result, err := s.powerRankedRetainedValue(value, exponent, usePrem)
	if err != nil {
		return err
	}
	return s.assignOwnedNumeralSlot(destination, result)
}
