package realclosure

// Source: src/math/realclosure/realclosure.cpp:4098-4128.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) evaluateRationalPolynomialInterval(coefficients []*realValue, argument, result *BinaryInterval) error {
	if len(coefficients) == 0 || coefficients[len(coefficients)-1] == nil || argument == nil || result == nil || argument.numbers != result.numbers {
		return arena.ErrInvalidSlot
	}
	bounds := argument.numbers
	coefficientInterval := func(value *realValue) (*BinaryInterval, error) {
		if value == nil || !value.isRational || value.store != s || value.deleted || value.refs == 0 {
			return nil, arena.ErrInvalidSlot
		}
		return s.cachedRationalApproxInterval(value, bounds)
	}
	leading, err := coefficientInterval(coefficients[len(coefficients)-1])
	if err != nil {
		return err
	}
	if len(coefficients) == 1 {
		return result.SetFrom(leading)
	}
	if err = result.Mul(leading, argument); err != nil {
		return err
	}
	for i := len(coefficients) - 2; i >= 0; i-- {
		if coefficients[i] != nil {
			coefficient, e := coefficientInterval(coefficients[i])
			if e != nil {
				return e
			}
			if e = result.Add(result, coefficient); e != nil {
				return e
			}
		}
		if i > 0 {
			if err = result.Mul(result, argument); err != nil {
				return err
			}
		}
	}
	return nil
}

func binaryIntervalPrecise(interval *BinaryInterval, precision uint32) (bool, error) {
	if interval == nil || interval.lowerInf || interval.upperInf {
		return false, nil
	}
	width := numeral.NewBinaryRational()
	if err := interval.numbers.Sub(&interval.upper, &interval.lower, &width); err != nil {
		return false, err
	}
	precise, err := interval.numbers.LessOneDiv2k(&width, precision)
	return precise, errorsJoinBinary(interval.numbers, &width, err)
}

func errorsJoinBinary(manager *numeral.BinaryRationalManager, value *numeral.BinaryRational, body error) error {
	return errors.Join(body, manager.Del(value))
}
