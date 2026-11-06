package realclosure

// Source: src/math/realclosure/realclosure.cpp:6294-6305.

import "errors"

func (s *realValueStore) setNumeralSlotInt64(destination **realValue, value int64) error {
	owned, err := s.newRational(value, 1)
	if err != nil {
		return err
	}
	err = s.copyNumeralSlot(destination, owned)
	return errors.Join(err, s.release(owned))
}
