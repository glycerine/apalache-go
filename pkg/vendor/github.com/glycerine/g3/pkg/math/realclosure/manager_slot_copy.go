package realclosure

// Source: src/math/realclosure/realclosure.cpp:5676-5680, 6306-6310.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) copyNumeralSlot(destination **realValue, source *realValue) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	if source != nil && (source.store != s || source.deleted || source.refs == 0) {
		return arena.ErrInvalidSlot
	}
	if *destination == source {
		return nil
	}
	if err := s.retain(source); err != nil {
		return err
	}
	old := *destination
	if err := s.release(old); err != nil {
		return errors.Join(err, s.release(source))
	}
	*destination = source
	return nil
}
