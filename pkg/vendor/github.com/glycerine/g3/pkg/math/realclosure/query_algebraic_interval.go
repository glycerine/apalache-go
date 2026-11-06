package realclosure

// Source: src/math/realclosure/realclosure.cpp:3440-3463.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

type algebraicIntervalQuery struct {
	LowerInfinite bool
	LowerOpen     bool
	Lower         *realValue
	UpperInfinite bool
	UpperOpen     bool
	Upper         *realValue
}

func (s *realValueStore) queryAlgebraicInterval(value *realValue) (result algebraicIntervalQuery, algebraic bool, err error) {
	if value == nil || value.isRational {
		return result, false, nil
	}
	if value.root != nil && value.root.kind != realExtensionAlgebraic {
		return result, false, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 || value.root == nil || value.root.isolating == nil {
		return result, false, arena.ErrInvalidSlot
	}
	if _, err = value.root.registryIndex(); err != nil {
		return result, false, err
	}
	interval := value.root.isolating
	if err = interval.open(); err != nil {
		return result, false, err
	}
	result.LowerInfinite, result.LowerOpen = interval.lowerInf, interval.lowerOpen
	result.UpperInfinite, result.UpperOpen = interval.upperInf, interval.upperOpen
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmScratch(result.Lower), s.releaseSturmScratch(result.Upper))
			result = algebraicIntervalQuery{}
		}
	}()
	if !result.LowerInfinite {
		result.Lower, err = s.ownBinaryBoundValue(interval.numbers, &interval.lower)
		if err != nil {
			return result, false, err
		}
	}
	if !result.UpperInfinite {
		result.Upper, err = s.ownBinaryBoundValue(interval.numbers, &interval.upper)
		if err != nil {
			return result, false, err
		}
	}
	return result, true, nil
}
