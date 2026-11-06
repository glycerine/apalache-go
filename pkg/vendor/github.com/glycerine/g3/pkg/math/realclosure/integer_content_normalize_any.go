package realclosure

// Source: src/math/realclosure/realclosure.cpp:3370-3458.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) normalizeAnyIntegerContent(p []*realValue, usePrem bool) (result []*realValue, normalized bool, err error) {
	gcd := numeral.NewInt(0)
	defer func() { err = errors.Join(err, s.releaseSturmInteger(&gcd)) }()
	clean, err := s.polynomialIntegerContent(p, &gcd)
	if err != nil {
		return nil, false, err
	}
	one, err := s.numbers.Integers().IsOne(&gcd)
	if err != nil {
		return nil, false, err
	}
	result = make([]*realValue, len(p))
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	if !clean || one {
		for i, coefficient := range p {
			if err = s.retain(coefficient); err != nil {
				return nil, false, err
			}
			result[i] = coefficient
		}
		return result, false, nil
	}
	for i, coefficient := range p {
		result[i], err = s.exactDivideValueByInteger(coefficient, &gcd, usePrem)
		if err != nil {
			return nil, false, err
		}
	}
	return result, true, nil
}
