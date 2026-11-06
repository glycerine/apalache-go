package realclosure

// Source: src/math/realclosure/realclosure.cpp:5419-5585, retained rational
// and coprime algebraic denominator-one path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) inverseRetainedValue(value *realValue) (result *realValue, err error) {
	if value == nil || s == nil || value.store != s || value.deleted || value.refs == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if value.isRational {
		one, err := s.newRational(1, 1)
		if err != nil {
			return nil, err
		}
		defer func() { err = errors.Join(err, s.releaseSturmScratch(one)) }()
		return s.divideRationals(one, value)
	}
	root := value.root
	inverse, coprime, err := s.invertRationalPolynomialModulo(value.functionNumerator.coefficients, root.polynomial.coefficients)
	if err != nil {
		return nil, err
	}
	if !coprime {
		return nil, arena.ErrInvalidSlot
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(inverse)) }()
	if len(inverse) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if len(inverse) == 1 {
		result = inverse[0]
		inverse[0] = nil
		return result, nil
	}
	approximation, err := NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	if err = approximation.Inv(value.functionInterval, s.rationalInitialPrecision()); err != nil {
		return nil, err
	}
	return s.newAlgebraicPolynomialValue(root, inverse, approximation)
}
