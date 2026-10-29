package realclosure

// Source: src/math/realclosure/realclosure.cpp:2917-2988,
// 3020-3043, and 3401-3420.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// signedPseudoRemainderRational follows prem, sprem, then the default
// Sturm-sequence integer-content normalization. The source's odd-step sign
// rule is retained for a negative leading divisor coefficient.
func (s *realValueStore) signedPseudoRemainderRational(left, right []*realValue) ([]*realValue, error) {
	if s == nil || s.numbers == nil || len(right) == 0 || right[len(right)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, value := range left {
		if !s.rationalOperand(value) {
			return nil, arena.ErrInvalidSlot
		}
	}
	for _, value := range right {
		if !s.rationalOperand(value) {
			return nil, arena.ErrInvalidSlot
		}
	}
	if len(right) == 1 {
		return nil, nil
	}
	result := append([]*realValue(nil), left...)
	for i, value := range result {
		if err := s.retain(value); err != nil {
			_ = s.releaseSturmCoefficients(result[:i])
			return nil, err
		}
	}
	leading := right[len(right)-1]
	one, err := s.numbers.IsOne(&leading.rational)
	if err != nil {
		_ = s.releaseSturmCoefficients(result)
		return nil, err
	}
	steps := 0
	for len(result) >= len(right) {
		steps++
		shift := len(result) - len(right)
		high := result[len(result)-1]
		if err := s.retain(high); err != nil {
			_ = s.releaseSturmCoefficients(result)
			return nil, err
		}
		if !one {
			for i := 0; i < len(result)-1; i++ {
				updated, err := s.multiplyRationals(result[i], leading)
				if err != nil {
					_ = s.releaseSturmScratch(high)
					_ = s.releaseSturmCoefficients(result)
					return nil, err
				}
				if err := s.release(result[i]); err != nil {
					_ = s.releaseSturmScratch(updated)
					_ = s.releaseSturmScratch(high)
					_ = s.releaseSturmCoefficients(result)
					return nil, err
				}
				result[i] = updated
			}
		}
		for i := 0; i < len(right)-1; i++ {
			product, err := s.multiplyRationals(high, right[i])
			if err != nil {
				_ = s.releaseSturmScratch(high)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			index := i + shift
			updated, err := s.subtractRationals(result[index], product)
			if err != nil {
				_ = s.releaseSturmScratch(product)
				_ = s.releaseSturmScratch(high)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			if err := s.release(result[index]); err != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(product)
				_ = s.releaseSturmScratch(high)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			if err := s.releaseSturmScratch(product); err != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(high)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			result[index] = updated
		}
		if err := s.releaseSturmScratch(high); err != nil {
			_ = s.releaseSturmCoefficients(result)
			return nil, err
		}
		if err := s.release(result[len(result)-1]); err != nil {
			_ = s.releaseSturmCoefficients(result)
			return nil, err
		}
		result = result[:len(result)-1]
		for len(result) > 0 && result[len(result)-1] == nil {
			result = result[:len(result)-1]
		}
	}
	leadingSign, err := s.numbers.Sign(&leading.rational)
	if err != nil {
		_ = s.releaseSturmCoefficients(result)
		return nil, err
	}
	if steps%2 == 0 || leadingSign > 0 {
		for i, value := range result {
			negated, err := s.negateRational(value)
			if err != nil {
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			if err := s.release(value); err != nil {
				_ = s.releaseSturmScratch(negated)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			result[i] = negated
		}
	}
	if err := s.normalizeRationalIntegerContent(result); err != nil {
		_ = s.releaseSturmCoefficients(result)
		return nil, err
	}
	return result, nil
}

func (s *realValueStore) normalizeRationalIntegerContent(values []*realValue) (resultErr error) {
	gcd := numeral.NewInt(0)
	defer func() {
		if err := s.releaseSturmInteger(&gcd); resultErr == nil {
			resultErr = err
		}
	}()
	for _, value := range values {
		if value == nil {
			continue
		}
		integer, err := s.numbers.IsInt(&value.rational)
		if err != nil {
			return err
		}
		if !integer {
			return nil
		}
		numerator := numeral.NewInt(0)
		if err := s.numbers.Numerator(&value.rational, &numerator); err != nil {
			_ = s.releaseSturmInteger(&numerator)
			return err
		}
		if err := s.numbers.Integers().GCD(&gcd, &numerator, &gcd); err != nil {
			_ = s.releaseSturmInteger(&numerator)
			return err
		}
		if err := s.releaseSturmInteger(&numerator); err != nil {
			return err
		}
	}
	if len(values) == 0 {
		return nil
	}
	one, err := s.numbers.Integers().IsOne(&gcd)
	if err != nil || one {
		return err
	}
	for i, value := range values {
		if value == nil {
			continue
		}
		out := numeral.NewRational()
		if err := s.numbers.DivInt(&value.rational, &gcd, &out); err != nil {
			_ = s.releaseSturmRational(&out)
			return err
		}
		updated, err := s.ownNonzeroRational(&out)
		if err != nil {
			return err
		}
		if err := s.release(value); err != nil {
			_ = s.releaseSturmScratch(updated)
			return err
		}
		values[i] = updated
	}
	return nil
}
