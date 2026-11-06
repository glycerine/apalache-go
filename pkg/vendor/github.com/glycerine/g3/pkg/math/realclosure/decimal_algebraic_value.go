package realclosure

// Source: src/math/realclosure/realclosure.cpp:6149-6164.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) displayAlgebraicValueDecimal(w io.Writer, value *realValue, bounds *numeral.BinaryRationalManager, precision uint32) error {
	if s == nil || w == nil || value == nil || value.store != s || value.deleted || value.refs == 0 || value.isRational {
		return arena.ErrInvalidSlot
	}
	refined, err := s.refineAnyRetainedInterval(value, bounds, precision*4)
	if err != nil {
		// Multi-root sign-determined extensions are retained, but their interval
		// bisection is not: follow the source unknown-decimal branch after exact
		// sign determination instead of treating that valid value as malformed.
		if value.root == nil || value.root.signs == nil || !errors.Is(err, arena.ErrInvalidSlot) {
			return err
		}
		refined = false
	}
	if !refined {
		var sign int32
		var signErr error
		if value.root != nil && value.root.kind != realExtensionAlgebraic {
			sign, signErr = s.determineRankedRetainedValueSign(value, true)
		} else {
			sign, signErr = s.determineAnyRetainedValueSign(value, true)
		}
		if signErr != nil {
			return signErr
		}
		if sign < 0 {
			return writeRealClosureString(w, "-?")
		}
		return writeRealClosureString(w, "?")
	}
	interval, err := s.valueInterval(value, bounds)
	if err != nil {
		return err
	}
	endpoint, err := decimalAlgebraicEndpoint(interval)
	if err != nil {
		return err
	}
	text, err := bounds.DecimalString(endpoint, precision)
	if err != nil {
		return err
	}
	return writeRealClosureString(w, text)
}
