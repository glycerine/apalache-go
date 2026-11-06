package subpaving

// Source: src/math/subpaving/subpaving_types.h:30-31 and the concrete
// wrapper and context throws in subpaving.cpp and subpaving_t_def.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrSubpavingFailure = errors.New("subpaving: source exception")
var ErrRationalContextTimestamp error = &sourceFailure{message: "subpaving: rational bound timestamp exhausted"}

type sourceFailure struct{ message string }

func (e *sourceFailure) Error() string { return e.message }
func (e *sourceFailure) Unwrap() error { return ErrSubpavingFailure }

// The source fpoint wrapper catches only its numeral-manager exception and
// replaces it with a message-free subpaving::exception.
func sourceFastFailure(err error) error {
	if errors.Is(err, numeral.ErrFastFloatDomain) {
		return ErrSubpavingFailure
	}
	return err
}

// The MPF wrapper catches f2n<mpf_manager>::exception around conversion,
// construction, and inequality publication.
func sourceArbitraryFailure(err error) error {
	if errors.Is(err, numeral.ErrArbitraryFloatNonRegular) {
		return ErrSubpavingFailure
	}
	return err
}

// The concrete MPFX wrapper catches its numeral-manager exception, including
// overflow, at coefficient, inequality and context construction boundaries.
func sourceFixedFailure(err error) error {
	if errors.Is(err, numeral.ErrFixedPointDomain) {
		return ErrSubpavingFailure
	}
	return err
}
