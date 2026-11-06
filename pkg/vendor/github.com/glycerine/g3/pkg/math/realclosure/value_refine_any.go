package realclosure

// Source: src/math/realclosure/realclosure.cpp:4313-4344.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	"math"
)

func (s *realValueStore) refineAnyRetainedInterval(value *realValue, bounds *numeral.BinaryRationalManager, precision uint32) (bool, error) {
	if err := s.checkpoint(); err != nil {
		return false, err
	}
	if value == nil {
		return true, nil
	}
	if value.store != s || value.deleted || value.refs == 0 || bounds == nil {
		return false, arena.ErrInvalidSlot
	}
	interval, err := s.valueInterval(value, bounds)
	if err != nil {
		return false, err
	}
	magnitude, err := s.sturmIntervalMagnitude(interval)
	if err != nil {
		return false, err
	}
	if magnitude == math.MinInt32 || magnitude < 0 && uint32(-magnitude) > precision {
		return true, nil
	}
	if err = s.saveIntervalIfTooSmall(value, precision); err != nil {
		return false, err
	}
	if value.isRational {
		return true, s.refineSturmRationalInterval(value, interval, precision)
	}
	if value.root != nil && value.root.kind == realExtensionTranscendental {
		return s.refineTranscendentalPolynomialValue(value, precision)
	}
	if value.root != nil && value.root.kind == realExtensionInfinitesimal {
		return s.refineInfinitesimalPolynomialValue(value, precision, s.usePseudoRemainder())
	}
	if value.root.owner.intervals != bounds {
		return false, arena.ErrInvalidSlot
	}
	return s.refineAlgebraicPolynomialValue(value, precision)
}
