package realclosure

// Source: src/math/realclosure/realclosure.cpp:2753-2791 and 1704-1741.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) rationalOperand(v *realValue) bool {
	return v == nil || v.store == s && v.isRational && !v.deleted && v.refs > 0
}

func (s *realValueStore) multiplyRationals(a, b *realValue) (*realValue, error) {
	if !s.rationalOperand(a) || !s.rationalOperand(b) {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil || b == nil {
		return nil, nil
	}
	out := numeral.NewRational()
	if err := s.numbers.Mul(&a.rational, &b.rational, &out); err != nil {
		_ = s.releaseSturmRational(&out)
		return nil, err
	}
	zero, err := s.numbers.IsZero(&out)
	if err != nil {
		_ = s.releaseSturmRational(&out)
		return nil, err
	}
	if zero {
		if err := s.releaseSturmRational(&out); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return &realValue{store: s, rational: out, refs: 1, isRational: true}, nil
}

func (s *realValueStore) addRationals(a, b *realValue) (*realValue, error) {
	if !s.rationalOperand(a) || !s.rationalOperand(b) {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil {
		if err := s.retain(b); err != nil {
			return nil, err
		}
		return b, nil
	}
	if b == nil {
		if err := s.retain(a); err != nil {
			return nil, err
		}
		return a, nil
	}
	out := numeral.NewRational()
	if err := s.numbers.Add(&a.rational, &b.rational, &out); err != nil {
		_ = s.releaseSturmRational(&out)
		return nil, err
	}
	zero, err := s.numbers.IsZero(&out)
	if err != nil {
		_ = s.releaseSturmRational(&out)
		return nil, err
	}
	if zero {
		if err := s.releaseSturmRational(&out); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return &realValue{store: s, rational: out, refs: 1, isRational: true}, nil
}

// multiplyRationalPolynomials follows the source's nested i,j convolution,
// including nil zero coefficients and trailing-zero trimming. Returned
// coefficient values carry one owning reference each.
func (s *realValueStore) multiplyRationalPolynomials(left, right []*realValue) ([]*realValue, error) {
	if s == nil || s.numbers == nil {
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
	result := make([]*realValue, len(left)*len(right))
	if len(left) < len(right) {
		left, right = right, left
	}
	for i, a := range left {
		if a == nil {
			continue
		}
		for j, b := range right {
			product, err := s.multiplyRationals(a, b)
			if err != nil {
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			sum, err := s.addRationals(result[i+j], product)
			if err != nil {
				_ = s.releaseSturmScratch(product)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			if err := s.release(result[i+j]); err != nil {
				_ = s.releaseSturmScratch(sum)
				_ = s.releaseSturmScratch(product)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			if err := s.releaseSturmScratch(product); err != nil {
				_ = s.releaseSturmScratch(sum)
				_ = s.releaseSturmCoefficients(result)
				return nil, err
			}
			result[i+j] = sum
		}
	}
	for len(result) > 0 && result[len(result)-1] == nil {
		result = result[:len(result)-1]
	}
	return result, nil
}

func (s *realValueStore) releaseCoefficients(values []*realValue) error {
	for _, value := range values {
		if err := s.release(value); err != nil {
			return err
		}
	}
	return nil
}
