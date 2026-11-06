package realclosure

// Source: src/math/realclosure/realclosure.cpp:6152-6156.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func decimalAlgebraicEndpoint(interval *BinaryInterval) (*numeral.BinaryRational, error) {
	if err := interval.open(); err != nil {
		return nil, err
	}
	if interval.lowerInf || interval.upperInf {
		return nil, arena.ErrInvalidSlot
	}
	integer, err := interval.numbers.IsInteger(&interval.lower)
	if err != nil {
		return nil, err
	}
	if integer {
		return &interval.upper, nil
	}
	return &interval.lower, nil
}
