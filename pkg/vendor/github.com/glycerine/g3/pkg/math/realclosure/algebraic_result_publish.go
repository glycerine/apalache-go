package realclosure

// Source: src/math/realclosure/realclosure.cpp:1220-1275, publish a reduced
// denominator-one polynomial result and collapse constants.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) publishAlgebraicPolynomialResult(root *algebraicRoot, coefficients []*realValue, approximation *BinaryInterval) (*realValue, error) {
	if root == nil || approximation == nil || len(coefficients) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if len(coefficients) == 1 {
		if err := s.retain(coefficients[0]); err != nil {
			return nil, err
		}
		return coefficients[0], nil
	}
	return s.newAlgebraicPolynomialValue(root, coefficients, approximation)
}
