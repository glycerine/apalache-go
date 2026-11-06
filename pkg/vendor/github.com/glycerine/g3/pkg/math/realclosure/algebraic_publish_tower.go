package realclosure

// Source: src/math/realclosure/realclosure.cpp:1220-1275, 3130-3165.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) publishRankedAlgebraicPolynomialResult(root *algebraicRoot, coefficients []*realValue, approximation *BinaryInterval) (*realValue, error) {
	if root == nil || approximation == nil || len(coefficients) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if len(coefficients) == 1 {
		if err := s.retain(coefficients[0]); err != nil {
			return nil, err
		}
		return coefficients[0], nil
	}
	return s.newRankedAlgebraicPolynomialValue(root, coefficients, approximation)
}

func (s *realValueStore) publishRankedPolynomialValue(root *algebraicRoot, coefficients []*realValue, usePrem bool) (value *realValue, err error) {
	if root == nil || len(coefficients) == 0 || coefficients[len(coefficients)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	if len(coefficients) == 1 {
		if err = s.retain(coefficients[0]); err != nil {
			return nil, err
		}
		return coefficients[0], nil
	}
	approximation, err := NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	if err = s.evaluateRankedPolynomialInterval(coefficients, root.interval, approximation); err != nil {
		return nil, err
	}
	return s.publishRankedAlgebraicPolynomialResult(root, coefficients, approximation)
}
