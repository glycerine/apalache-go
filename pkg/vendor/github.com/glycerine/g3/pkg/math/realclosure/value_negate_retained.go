package realclosure

// Source: src/math/realclosure/realclosure.cpp:4808-4865.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) negateRetainedValue(value *realValue) (result *realValue, err error) {
	if s == nil || !s.rationalOperand(value) && (value == nil || value.store != s || value.deleted || value.refs == 0 || value.root == nil) {
		return nil, arena.ErrInvalidSlot
	}
	if value == nil || value.isRational {
		return s.negateRational(value)
	}
	numerator := make([]*realValue, len(value.functionNumerator.coefficients))
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(numerator)) }()
	for i, coefficient := range value.functionNumerator.coefficients {
		numerator[i], err = s.negateRational(coefficient)
		if err != nil {
			return nil, err
		}
	}
	approximation, err := NewBinaryInterval(value.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	if err = approximation.Neg(value.functionInterval); err != nil {
		return nil, err
	}
	return s.newAlgebraicPolynomialValue(value.root, numerator, approximation)
}
