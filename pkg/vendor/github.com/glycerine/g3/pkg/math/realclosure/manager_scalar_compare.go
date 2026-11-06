package realclosure

// Source: src/math/realclosure/realclosure.cpp:6192-6239.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) compareValueRational(value *realValue, scalar *numeral.Rational) (comparison int, err error) {
	if s == nil || s.numbers == nil || scalar == nil {
		return 0, arena.ErrInvalidSlot
	}
	if err = s.numbers.Check(scalar); err != nil {
		return 0, err
	}
	temporary, err := s.newRationalCopy(scalar)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.release(temporary)) }()
	return s.compareValues(value, temporary, nil)
}

func (s *realValueStore) compareValueInteger(value *realValue, scalar *numeral.Int) (comparison int, err error) {
	if s == nil || s.numbers == nil || scalar == nil || s.numbers.Integers() == nil {
		return 0, arena.ErrInvalidSlot
	}
	if err = s.numbers.Integers().Check(scalar); err != nil {
		return 0, err
	}
	temporary := numeral.NewRational()
	defer func() { err = errors.Join(err, s.numbers.Del(&temporary)) }()
	if err = s.numbers.SetInteger(&temporary, scalar); err != nil {
		return 0, err
	}
	return s.compareValueRational(value, &temporary)
}
