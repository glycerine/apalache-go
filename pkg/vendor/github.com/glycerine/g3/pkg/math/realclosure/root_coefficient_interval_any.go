package realclosure

// Source: src/math/realclosure/realclosure.cpp:1383-1456.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) coefficientApproxIntervalAny(value *realValue, bounds *numeral.BinaryRationalManager) (*BinaryInterval, error) {
	if value == nil || bounds == nil || value.store != s || value.deleted || value.refs == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.rationalApproxInterval(value, bounds)
	}
	if value.functionInterval == nil || value.functionInterval.numbers != bounds {
		return nil, arena.ErrInvalidSlot
	}
	copy, err := NewBinaryInterval(bounds)
	if err != nil {
		return nil, err
	}
	if err = copy.SetFrom(value.functionInterval); err != nil {
		_ = copy.Close()
		return nil, err
	}
	return copy, nil
}

func (s *realValueStore) coefficientMagnitudeAny(value *realValue, bounds *numeral.BinaryRationalManager, lower bool) (int32, bool, error) {
	interval, err := s.coefficientApproxIntervalAny(value, bounds)
	if err != nil {
		return 0, false, err
	}
	var magnitude int32
	var ok bool
	if lower {
		magnitude, ok, err = absLowerMagnitude(interval)
	} else {
		magnitude, ok, err = absUpperMagnitude(interval)
	}
	closeErr := interval.Close()
	if err != nil {
		return 0, false, err
	}
	return magnitude, ok, closeErr
}
