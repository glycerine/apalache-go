package realclosure

// Source: src/math/realclosure/realclosure.cpp:3216-3327.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) cleanTowerPolynomialDenominators(p []*realValue, usePrem bool) (normalized []*realValue, denominator *realValue, err error) {
	if len(p) == 0 {
		return nil, nil, arena.ErrInvalidSlot
	}
	numerators := make([]*realValue, len(p))
	denominators := make([]*realValue, len(p))
	defer func() {
		err = errors.Join(err, s.releaseSturmCoefficients(numerators), s.releaseSturmCoefficients(denominators))
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(normalized), s.releaseSturmScratch(denominator))
			normalized, denominator = nil, nil
		}
	}()
	for i, coefficient := range p {
		numerators[i], denominators[i], err = s.cleanTowerValueDenominator(coefficient, usePrem)
		if err != nil {
			return nil, nil, err
		}
	}
	denominator, err = s.lcmIntegerDenominators(denominators)
	if err != nil {
		return nil, nil, err
	}
	normalized = make([]*realValue, len(p))
	for i := range p {
		if numerators[i] == nil {
			continue
		}
		multiplier, e := s.divideRankedRetainedValues(denominator, denominators[i], usePrem)
		if e != nil {
			return nil, nil, e
		}
		normalized[i], e = s.multiplyRankedRetainedValues(numerators[i], multiplier, usePrem)
		e = errors.Join(e, s.releaseSturmScratch(multiplier))
		if e != nil {
			return nil, nil, e
		}
	}
	return normalized, denominator, nil
}
