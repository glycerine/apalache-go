package realclosure

// Source: src/math/realclosure/realclosure.cpp:2578-2586, 2638-2653.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) newRationalCopy(source *numeral.Rational) (value *realValue, err error) {
	if s == nil || s.numbers == nil || source == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err = s.numbers.Check(source); err != nil {
		return nil, err
	}
	zero, err := s.numbers.IsZero(source)
	if err != nil || zero {
		return nil, err
	}
	value = &realValue{store: s, rational: numeral.NewRational(), refs: 1, isRational: true}
	if err = s.numbers.Copy(&value.rational, source); err != nil {
		return nil, errors.Join(err, s.numbers.Del(&value.rational))
	}
	return value, nil
}
