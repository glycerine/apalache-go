package realclosure

// Source: src/math/realclosure/realclosure.cpp:5090-5138.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) reduceRankedAlgebraicPolynomial(root *algebraicRoot, numerator []*realValue, usePrem bool) (result []*realValue, err error) {
	if root == nil || root.deleted || root.owner == nil || root.owner.values != s {
		return nil, arena.ErrInvalidSlot
	}
	if root.kind != realExtensionAlgebraic {
		return s.retainRationalPolynomial(numerator)
	}
	if len(root.polynomial.coefficients) < 2 {
		return nil, arena.ErrInvalidSlot
	}
	if len(numerator) < len(root.polynomial.coefficients) {
		return s.retainRationalPolynomial(numerator)
	}
	quotient, remainder, err := s.divideAnyPolynomials(numerator, root.polynomial.coefficients, usePrem)
	if err != nil {
		return nil, err
	}
	if err = s.releaseSturmCoefficients(quotient); err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(remainder))
	}
	return remainder, nil
}
