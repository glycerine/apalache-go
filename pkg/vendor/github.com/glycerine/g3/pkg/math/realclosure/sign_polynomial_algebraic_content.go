package realclosure

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) integerContentValue(value *realValue, gcd *big.Int) bool {
	if value == nil {
		return true
	}
	if !s.realOperand(value) {
		return false
	}
	if !value.isRational {
		for _, coefficient := range value.functionNumerator.coefficients {
			if !s.integerContentValue(coefficient, gcd) {
				return false
			}
		}
		return true
	}
	text, err := s.numbers.String(&value.rational)
	if err != nil {
		return false
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok || q.Denom().Cmp(big.NewInt(1)) != 0 {
		return false
	}
	magnitude := new(big.Int).Abs(q.Num())
	gcd.GCD(nil, nil, gcd, magnitude)
	return true
}

func (s *realValueStore) normalizeIntegerContentNormalized(values []*realValue) error {
	var gcd big.Int
	for _, value := range values {
		if !s.integerContentValue(value, &gcd) {
			return nil
		}
		if gcd.Cmp(big.NewInt(1)) == 0 {
			return nil
		}
	}
	if gcd.Sign() == 0 || gcd.Cmp(big.NewInt(1)) == 0 {
		return nil
	}
	coefficient, err := s.ownBigPolynomial([]big.Rat{*new(big.Rat).SetInt(&gcd)})
	if err != nil || len(coefficient) != 1 {
		return arena.ErrInvalidSlot
	}
	divisor := coefficient[0]
	for i, value := range values {
		if value == nil {
			continue
		}
		updated, cause := s.divideNormalizedValues(value, divisor)
		if cause != nil {
			return errors.Join(cause, s.release(divisor))
		}
		if cause = s.release(value); cause != nil {
			_ = s.release(updated)
			return errors.Join(cause, s.release(divisor))
		}
		values[i] = updated
	}
	return s.release(divisor)
}
