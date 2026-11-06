package realclosure

// Source: src/math/realclosure/realclosure.h:296-320 and
// src/math/realclosure/realclosure.cpp:2623-2636.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) newIntegerValue(source *numeral.Int) (value *realValue, err error) {
	if s == nil || s.numbers == nil || s.numbers.Integers() == nil || source == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err = s.numbers.Integers().Check(source); err != nil {
		return nil, err
	}
	rational := numeral.NewRational()
	defer func() { err = errors.Join(err, s.numbers.Del(&rational)) }()
	if err = s.numbers.SetInteger(&rational, source); err != nil {
		return nil, err
	}
	return s.newRationalCopy(&rational)
}
