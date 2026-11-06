package realclosure

// Source: src/math/realclosure/realclosure.cpp:5150-5318, retained rational
// and same algebraic-extension denominator-one paths.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) multiplyRetainedValues(left, right *realValue) (result *realValue, err error) {
	if !s.rationalOperand(left) && (left == nil || left.store != s || left.deleted || left.refs == 0) || !s.rationalOperand(right) && (right == nil || right.store != s || right.deleted || right.refs == 0) {
		return nil, arena.ErrInvalidSlot
	}
	if left == nil || right == nil {
		return nil, nil
	}
	if left.isRational && right.isRational {
		return s.multiplyRationals(left, right)
	}
	var root *algebraicRoot
	leftCoefficients := []*realValue{left}
	rightCoefficients := []*realValue{right}
	if !left.isRational {
		root = left.root
		leftCoefficients = left.functionNumerator.coefficients
	}
	if !right.isRational {
		if root != nil && right.root != root {
			return nil, arena.ErrInvalidSlot
		}
		root = right.root
		rightCoefficients = right.functionNumerator.coefficients
	}
	if root == nil || root.deleted {
		return nil, arena.ErrInvalidSlot
	}
	product, err := s.multiplyRationalPolynomials(leftCoefficients, rightCoefficients)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(product)) }()
	reduced, err := s.reduceAlgebraicPolynomial(root, product)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(reduced)) }()
	if len(reduced) == 0 {
		return nil, nil
	}
	if len(reduced) == 1 {
		result = reduced[0]
		reduced[0] = nil
		return result, nil
	}
	bounds := root.owner.intervals
	leftInterval, err := s.valueInterval(left, bounds)
	if err != nil {
		return nil, err
	}
	rightInterval, err := s.valueInterval(right, bounds)
	if err != nil {
		return nil, err
	}
	approximation, err := NewBinaryInterval(bounds)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	if err = approximation.Mul(leftInterval, rightInterval); err != nil {
		return nil, err
	}
	return s.newAlgebraicPolynomialValue(root, reduced, approximation)
}
