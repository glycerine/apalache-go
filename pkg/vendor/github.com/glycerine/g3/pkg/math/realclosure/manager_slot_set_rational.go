package realclosure

// Source: src/math/realclosure/realclosure.cpp:2638-2653, 6124-6126.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) setNumeralSlotRational(destination **realValue, source *numeral.Rational) error {
	if err := s.validateNumeralSlot(destination); err != nil {
		return err
	}
	owned, err := s.newRationalCopy(source)
	if err != nil {
		return err
	}
	err = s.copyNumeralSlot(destination, owned)
	return errors.Join(err, s.release(owned))
}
