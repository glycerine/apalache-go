package realclosure

// Source: src/math/realclosure/realclosure.cpp:6167-6180 and manager wrapper
// at 6430-6432.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) displayValueDecimal(w io.Writer, value *realValue, bounds *numeral.BinaryRationalManager, precision uint32) error {
	if s == nil || w == nil || bounds == nil {
		return arena.ErrInvalidSlot
	}
	if value == nil {
		return writeRealClosureString(w, "0")
	}
	if value.store != s || value.deleted || value.refs == 0 {
		return arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.displayRationalValueDecimal(w, value, precision)
	}
	return s.displayAlgebraicValueDecimal(w, value, bounds, precision)
}
