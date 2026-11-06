package realclosure

// Source: src/math/realclosure/realclosure.cpp:3390-3458 scalar owner.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) ownPositiveIntegerScalar(value *numeral.Int) (*realValue, error) {
	rational := numeral.NewRational()
	if err := s.numbers.SetInteger(&rational, value); err != nil {
		return nil, errors.Join(err, s.releaseSturmRational(&rational))
	}
	return s.ownNonzeroRational(&rational)
}
