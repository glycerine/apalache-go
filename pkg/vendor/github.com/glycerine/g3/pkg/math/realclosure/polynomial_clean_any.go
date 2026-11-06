package realclosure

// Source: src/math/realclosure/realclosure.cpp:3167-3288, retained values
// whose cleaned denominators are rational integers.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) cleanAnyPolynomialDenominators(p []*realValue, usePrem bool) (normalized []*realValue, denominator *realValue, err error) {
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
		numerators[i], denominators[i], err = s.cleanAnyValueDenominator(coefficient, usePrem)
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
		multiplier, e := s.divideRetainedValues(denominator, denominators[i])
		if e != nil {
			return nil, nil, e
		}
		normalized[i], e = s.multiplyAnyCanonicalValues(numerators[i], multiplier, usePrem)
		e = errors.Join(e, s.releaseSturmScratch(multiplier))
		if e != nil {
			return nil, nil, e
		}
	}
	return normalized, denominator, nil
}
