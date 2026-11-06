package realclosure

// Source: src/math/realclosure/realclosure.cpp:1180-1275, 4923-4943.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) newRankedAlgebraicPolynomialValue(root *algebraicRoot, numerator []*realValue, approximation *BinaryInterval) (value *realValue, err error) {
	var denominator []*realValue
	var one *realValue
	if root != nil && root.kind != realExtensionAlgebraic {
		one, err = s.newRational(1, 1)
		if err != nil {
			return nil, err
		}
		defer func() { err = errors.Join(err, s.releaseSturmScratch(one)) }()
		denominator = []*realValue{one}
	}
	return s.newRankedRationalFunctionValue(root, numerator, denominator, approximation)
}

func (s *realValueStore) newRankedRationalFunctionValue(root *algebraicRoot, numerator, denominator []*realValue, approximation *BinaryInterval) (value *realValue, err error) {
	if s == nil || root == nil || !root.isRegistered() || root.owner.values != s || approximation == nil || approximation.numbers != root.owner.intervals || len(numerator) == 0 || (root.kind == realExtensionAlgebraic && len(numerator) >= len(root.polynomial.coefficients)) || numerator[len(numerator)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	if root.kind == realExtensionAlgebraic && len(denominator) != 0 {
		return nil, arena.ErrInvalidSlot
	}
	if root.kind != realExtensionAlgebraic && (len(denominator) == 0 || denominator[len(denominator)-1] == nil) {
		return nil, arena.ErrInvalidSlot
	}
	for _, polynomial := range [][]*realValue{numerator, denominator} {
		for _, coefficient := range polynomial {
			if coefficient == nil {
				continue
			}
			if !s.valueHasLowerExtensionRank(coefficient, root) {
				return nil, arena.ErrInvalidSlot
			}
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
			err = errors.Join(err, s.resetPolynomial(&value.functionNumerator), s.resetPolynomial(&value.functionDenominator), root.release())
			value = nil
		}
	}()
	if err = s.setPolynomial(&value.functionNumerator, numerator); err != nil {
		return nil, err
	}
	if len(denominator) != 0 {
		if err = s.setPolynomial(&value.functionDenominator, denominator); err != nil {
			return nil, err
		}
	}
	value.functionInterval, err = NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	err = value.functionInterval.SetFrom(approximation)
	return value, err
}

func (s *realValueStore) valueHasLowerExtensionRank(value *realValue, root *algebraicRoot) bool {
	if value == nil {
		return true
	}
	if _, err := s.retainedValueRank(value); err != nil {
		return false
	}
	if value.isRational {
		return true
	}
	left, right := sourceExtensionRank(value.root.kind), sourceExtensionRank(root.kind)
	return left < right || left == right && value.root.index < root.index
}
