package realclosure

// Source: src/math/realclosure/realclosure.cpp:4867-5055, concrete live
// rational and same-extension denominator-one algebraic additions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) addRetainedValues(left, right *realValue) (result *realValue, err error) {
	if !s.rationalOperand(left) && (left == nil || left.store != s || left.deleted || left.refs == 0) || !s.rationalOperand(right) && (right == nil || right.store != s || right.deleted || right.refs == 0) {
		return nil, arena.ErrInvalidSlot
	}
	if left == nil {
		if err := s.retain(right); err != nil {
			return nil, err
		}
		return right, nil
	}
	if right == nil {
		if err := s.retain(left); err != nil {
			return nil, err
		}
		return left, nil
	}
	if left.isRational && right.isRational {
		return s.addRationals(left, right)
	}
	var root *algebraicRoot
	var leftCoefficients, rightCoefficients []*realValue
	if !left.isRational {
		root = left.root
		leftCoefficients = left.functionNumerator.coefficients
	} else {
		leftCoefficients = []*realValue{left}
	}
	if !right.isRational {
		if root != nil && right.root != root {
			return nil, arena.ErrInvalidSlot
		}
		root = right.root
		rightCoefficients = right.functionNumerator.coefficients
	} else {
		rightCoefficients = []*realValue{right}
	}
	if root == nil {
		return nil, arena.ErrInvalidSlot
	}
	size := len(leftCoefficients)
	if len(rightCoefficients) > size {
		size = len(rightCoefficients)
	}
	numerator := make([]*realValue, size)
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(numerator)) }()
	for i := 0; i < size; i++ {
		var a, b *realValue
		if i < len(leftCoefficients) {
			a = leftCoefficients[i]
		}
		if i < len(rightCoefficients) {
			b = rightCoefficients[i]
		}
		numerator[i], err = s.addRationals(a, b)
		if err != nil {
			return nil, err
		}
	}
	for len(numerator) > 0 && numerator[len(numerator)-1] == nil {
		numerator = numerator[:len(numerator)-1]
	}
	if len(numerator) == 0 {
		return nil, nil
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
	if err = approximation.Add(leftInterval, rightInterval); err != nil {
		return nil, err
	}
	return s.newAlgebraicPolynomialValue(root, numerator, approximation)
}
