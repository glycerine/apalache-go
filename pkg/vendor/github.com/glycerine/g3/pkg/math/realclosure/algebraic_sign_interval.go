package realclosure

// Source: src/math/realclosure/realclosure.cpp:4570-4766 sign-certified
// interval update after the exact single-root Tarski query.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func restrictBinaryIntervalToSign(interval *BinaryInterval, sign int32) error {
	if interval == nil || sign == 0 {
		return arena.ErrInvalidSlot
	}
	if err := interval.open(); err != nil {
		return err
	}
	zero := numeral.NewBinaryRational()
	if err := interval.numbers.SetInt32(&zero, 0); err != nil {
		return err
	}
	defer func() { _ = interval.numbers.Del(&zero) }()
	if sign > 0 {
		if !interval.upperInf {
			positive, err := interval.numbers.Less(&zero, &interval.upper)
			if err != nil || !positive {
				return arena.ErrInvalidSlot
			}
		}
		if err := interval.numbers.Copy(&interval.lower, &zero); err != nil {
			return err
		}
		interval.lowerInf, interval.lowerOpen = false, true
	} else {
		if !interval.lowerInf {
			negative, err := interval.numbers.Less(&interval.lower, &zero)
			if err != nil || !negative {
				return arena.ErrInvalidSlot
			}
		}
		if err := interval.numbers.Copy(&interval.upper, &zero); err != nil {
			return err
		}
		interval.upperInf, interval.upperOpen = false, true
	}
	return interval.CheckInvariant()
}
