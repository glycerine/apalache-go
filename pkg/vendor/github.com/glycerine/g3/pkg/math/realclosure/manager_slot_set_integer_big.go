package realclosure

// Source: src/math/realclosure/realclosure.cpp:2623-2636, 6120-6122.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) setNumeralSlotInteger(destination **realValue, source *numeral.Int) (err error) {
	if err = s.validateNumeralSlot(destination); err != nil {
		return err
	}
	if s == nil || s.numbers == nil || source == nil {
		return arena.ErrInvalidSlot
	}
	integers := s.numbers.Integers()
	if integers == nil {
		return arena.ErrInvalidSlot
	}
	if err = integers.Check(source); err != nil {
		return err
	}
	rational := numeral.NewRational()
	defer func() { err = errors.Join(err, s.numbers.Del(&rational)) }()
	if err = s.numbers.SetInteger(&rational, source); err != nil {
		return err
	}
	return s.setNumeralSlotRational(destination, &rational)
}
