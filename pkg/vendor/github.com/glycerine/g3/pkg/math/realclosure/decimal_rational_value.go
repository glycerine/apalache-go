package realclosure

// Source: src/math/realclosure/realclosure.cpp:6171-6176.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) displayRationalValueDecimal(w io.Writer, value *realValue, precision uint32) error {
	if s == nil || s.numbers == nil || w == nil || value == nil || value.store != s || value.deleted || value.refs == 0 || !value.isRational {
		return arena.ErrInvalidSlot
	}
	return s.numbers.DisplayDecimalInto(realDecimalWriter{writer: w}, &value.rational, precision, false)
}
