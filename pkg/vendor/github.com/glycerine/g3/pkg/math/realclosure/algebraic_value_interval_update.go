package realclosure

// Source: src/math/realclosure/realclosure.cpp:4130-4147.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) updateAlgebraicPolynomialValueInterval(value *realValue) error {
	if value == nil || value.isRational || value.root == nil || value.functionInterval == nil || value.root.interval == nil {
		return arena.ErrInvalidSlot
	}
	return s.evaluateRationalPolynomialInterval(value.functionNumerator.coefficients, value.root.interval, value.functionInterval)
}
