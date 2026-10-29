package realclosure

// Source: src/math/realclosure/realclosure.cpp:2879-2913, 2991-3030.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) ownNonzeroRational(out *numeral.Rational) (*realValue, error) {
	zero, err := s.numbers.IsZero(out)
	if err != nil {
		return nil, errors.Join(err, s.releaseSturmRational(out))
	}
	if zero {
		if err := s.releaseSturmRational(out); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return &realValue{store: s, rational: *out, refs: 1, isRational: true}, nil
}

func (s *realValueStore) divideRationals(a, b *realValue) (*realValue, error) {
	if !s.rationalOperand(a) || !s.rationalOperand(b) || b == nil {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil {
		return nil, nil
	}
	out := numeral.NewRational()
	if err := s.numbers.Div(&a.rational, &b.rational, &out); err != nil {
		_ = s.releaseSturmRational(&out)
		return nil, err
	}
	return s.ownNonzeroRational(&out)
}

func (s *realValueStore) negateRational(a *realValue) (*realValue, error) {
	if !s.rationalOperand(a) {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil {
		return nil, nil
	}
	out := numeral.NewRational()
	if err := s.numbers.Copy(&out, &a.rational); err != nil {
		_ = s.releaseSturmRational(&out)
		return nil, err
	}
	if err := s.numbers.Neg(&out); err != nil {
		_ = s.releaseSturmRational(&out)
		return nil, err
	}
	return s.ownNonzeroRational(&out)
}

func (s *realValueStore) subtractRationals(a, b *realValue) (*realValue, error) {
	if !s.rationalOperand(a) || !s.rationalOperand(b) {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil {
		return s.negateRational(b)
	}
	if b == nil {
		if err := s.retain(a); err != nil {
			return nil, err
		}
		return a, nil
	}
	out := numeral.NewRational()
	if err := s.numbers.Sub(&a.rational, &b.rational, &out); err != nil {
		_ = s.releaseSturmRational(&out)
		return nil, err
	}
	return s.ownNonzeroRational(&out)
}

// signedRemainderRational follows rem's high-coefficient elimination and
// srem's final negation. Returned coefficients own one reference each.
func (s *realValueStore) signedRemainderRational(left, right []*realValue) ([]*realValue, error) {
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
	for len(result) >= len(right) {
		shift := len(result) - len(right)
		ratio, err := s.divideRationals(result[len(result)-1], right[len(right)-1])
		if err != nil {
			_ = s.releaseSturmCoefficients(result)
			return nil, err
		}
		for i := 0; i < len(right)-1; i++ {
			product, err := s.multiplyRationals(ratio, right[i])
			if err != nil {
				_ = s.releaseSturmScratch(ratio)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			index := i + shift
			updated, err := s.subtractRationals(result[index], product)
			if err != nil {
				_ = s.releaseSturmScratch(product)
				_ = s.releaseSturmScratch(ratio)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			if err := s.release(result[index]); err != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(product)
				_ = s.releaseSturmScratch(ratio)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			if err := s.releaseSturmScratch(product); err != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(ratio)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			result[index] = updated
		}
		if err := s.releaseSturmScratch(ratio); err != nil {
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
	return result, nil
}
