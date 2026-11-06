package realclosure

// Source: src/math/realclosure/realclosure.cpp:3290-3340.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) accumulateIntegerContent(value *realValue, gcd *numeral.Int) (bool, error) {
	if value == nil {
		return false, nil
	}
	if value.isRational {
		integer, err := s.numbers.IsInt(&value.rational)
		if err != nil || !integer {
			return false, err
		}
		numerator := numeral.NewInt(0)
		if err = s.numbers.Numerator(&value.rational, &numerator); err != nil {
			return false, errors.Join(err, s.releaseSturmInteger(&numerator))
		}
		err = s.numbers.Integers().GCD(gcd, &numerator, gcd)
		return err == nil, errors.Join(err, s.releaseSturmInteger(&numerator))
	}
	for _, coefficient := range value.functionNumerator.coefficients {
		if coefficient == nil {
			continue
		}
		clean, err := s.accumulateIntegerContent(coefficient, gcd)
		if err != nil || !clean {
			return clean, err
		}
	}
	return true, nil
}
