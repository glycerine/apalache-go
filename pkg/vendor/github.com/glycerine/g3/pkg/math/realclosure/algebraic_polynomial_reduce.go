package realclosure

// Source: src/math/realclosure/realclosure.cpp:5090-5138 normalize_algebraic,
// concrete rational-coefficient defining polynomial.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) reduceAlgebraicPolynomial(root *algebraicRoot, numerator []*realValue) (result []*realValue, err error) {
	if root == nil || root.deleted || root.owner == nil || root.owner.values != s || len(root.polynomial.coefficients) < 2 {
		return nil, arena.ErrInvalidSlot
	}
	normalize, err := s.shouldNormalizeAlgebraic(root)
	if err != nil {
		return nil, err
	}
	if !normalize {
		return s.retainRationalPolynomial(numerator)
	}
	if err = s.checkRationalPolynomial(numerator, true); err != nil {
		return nil, err
	}
	if len(numerator) < len(root.polynomial.coefficients) {
		result = append([]*realValue(nil), numerator...)
		for i, coefficient := range result {
			if err = s.retain(coefficient); err != nil {
				_ = s.releaseSturmCoefficients(result[:i])
				return nil, err
			}
		}
		return result, nil
	}
	quotient, remainder, err := s.divideRationalPolynomial(numerator, root.polynomial.coefficients)
	if err != nil {
		return nil, err
	}
	if err = s.releaseSturmCoefficients(quotient); err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(remainder))
	}
	return remainder, nil
}
