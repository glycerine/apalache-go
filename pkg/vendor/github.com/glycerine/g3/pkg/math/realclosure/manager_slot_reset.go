package realclosure

// Source: src/math/realclosure/realclosure.cpp:2440-2443, 6251-6255.

func (s *realValueStore) resetNumeralSlot(slot **realValue) error {
	return s.copyNumeralSlot(slot, nil)
}
