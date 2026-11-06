package realclosure

// Source: src/math/realclosure/realclosure.cpp:1180-1275, concrete
// denominator-one algebraic rational-function value.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) newAlgebraicPolynomialValue(root *algebraicRoot, numerator []*realValue, approximation *BinaryInterval) (value *realValue, err error) {
	if s == nil || root == nil || root.deleted || root.owner == nil || root.owner.values != s || approximation == nil || approximation.numbers != root.owner.intervals || len(numerator) == 0 || len(numerator) >= len(root.polynomial.coefficients) || numerator[len(numerator)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, coefficient := range numerator {
		if !s.rationalOperand(coefficient) {
			return nil, arena.ErrInvalidSlot
		}
	}
	if err = root.retain(); err != nil {
		return nil, err
	}
	value = &realValue{store: s, root: root, refs: 1}
	defer func() {
		if err != nil {
			if value.functionInterval != nil {
				err = errors.Join(err, value.functionInterval.Close())
			}
			err = errors.Join(err, s.resetPolynomial(&value.functionNumerator), root.release())
			value = nil
		}
	}()
	if err = s.setPolynomial(&value.functionNumerator, numerator); err != nil {
		return nil, err
	}
	value.functionInterval, err = NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	err = value.functionInterval.SetFrom(approximation)
	return value, err
}
