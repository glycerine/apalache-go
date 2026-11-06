package realclosure

// Source: src/math/realclosure/realclosure.cpp:3978-3992.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// ownBinaryBoundValue converts the exact c/2^k bound to the retained rational
// value used by the source's exact Horner fallback. The source scales the
// complete Horner sequence by powers of 2^k; retaining c/2^k gives the same
// exact value and sign while reusing the concrete rational owner.
func (s *realValueStore) ownBinaryBoundValue(bounds *numeral.BinaryRationalManager, bound *numeral.BinaryRational) (*realValue, error) {
	if s == nil || s.numbers == nil || bounds == nil || bound == nil {
		return nil, arena.ErrInvalidSlot
	}
	value := numeral.NewRational()
	if err := bounds.ToRational(s.numbers, bound, &value); err != nil {
		return nil, errors.Join(err, s.releaseSturmRational(&value))
	}
	return s.ownNonzeroRational(&value)
}
