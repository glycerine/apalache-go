package realclosure

// Source: src/math/realclosure/realclosure.cpp:5163-5318.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) multiplyRankedRetainedValues(left, right *realValue, usePrem bool) (result *realValue, err error) {
	comparison, err := s.compareRetainedValueRanks(left, right)
	if err != nil {
		return nil, err
	}
	if left == nil || right == nil {
		return nil, nil
	}
	if left.isRational && right.isRational {
		return s.multiplyRationals(left, right)
	}
	higher, lower := left, right
	if comparison < 0 {
		higher, lower = right, left
	}
	root := higher.root
	leftCoefficients := higher.functionNumerator.coefficients
	rightCoefficients := []*realValue{lower}
	if comparison == 0 {
		leftCoefficients = left.functionNumerator.coefficients
		rightCoefficients = right.functionNumerator.coefficients
	}
	product := make([]*realValue, len(leftCoefficients)+len(rightCoefficients)-1)
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(product)) }()
	for i, a := range leftCoefficients {
		for j, b := range rightCoefficients {
			term, e := s.multiplyRankedRetainedValues(a, b, usePrem)
			if e != nil {
				return nil, e
			}
			updated, e := s.addRankedRetainedValues(product[i+j], term, usePrem)
			e = errors.Join(e, s.releaseSturmScratch(term))
			if e != nil {
				return nil, e
			}
			if e = s.releaseSturmScratch(product[i+j]); e != nil {
				_ = s.releaseSturmScratch(updated)
				return nil, e
			}
			product[i+j] = updated
		}
	}
	product = trimAnyPolynomial(product)
	reduced, err := s.reduceRankedAlgebraicPolynomial(root, product, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(reduced)) }()
	if len(reduced) == 0 {
		return nil, nil
	}
	var denominator []*realValue
	ownedDenominator := false
	if root.kind != realExtensionAlgebraic {
		if comparison == 0 {
			denominator, err = s.multiplyRankedPolynomials(left.functionDenominator.coefficients, right.functionDenominator.coefficients, usePrem)
			if err != nil {
				return nil, err
			}
			ownedDenominator = true
		} else {
			denominator = higher.functionDenominator.coefficients
		}
		if len(denominator) == 0 {
			if ownedDenominator {
				_ = s.releaseSturmCoefficients(denominator)
			}
			return nil, arena.ErrInvalidSlot
		}
	}
	if ownedDenominator {
		defer func() { err = errors.Join(err, s.releaseSturmCoefficients(denominator)) }()
	}
	if len(reduced) == 1 && (root.kind == realExtensionAlgebraic || s.rankedDenominatorIsOne(denominator)) {
		result = reduced[0]
		reduced[0] = nil
		return result, nil
	}
	approximation, err := NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	leftInterval, err := s.valueInterval(left, root.owner.intervals)
	if err != nil {
		return nil, err
	}
	rightInterval, err := s.valueInterval(right, root.owner.intervals)
	if err != nil {
		return nil, err
	}
	if err = approximation.Mul(leftInterval, rightInterval); err != nil {
		return nil, err
	}
	if root.kind == realExtensionAlgebraic {
		result, err = s.newRankedAlgebraicPolynomialValue(root, reduced, approximation)
	} else {
		var normalizedNumerator, normalizedDenominator []*realValue
		normalizedNumerator, normalizedDenominator, err = s.normalizeRankedFraction(reduced, denominator, usePrem)
		if err == nil {
			defer func() {
				err = errors.Join(err, s.releaseSturmCoefficients(normalizedNumerator), s.releaseSturmCoefficients(normalizedDenominator))
			}()
			result, err = s.newRankedRationalFunctionValue(root, normalizedNumerator, normalizedDenominator, approximation)
		}
	}
	if err != nil {
		return nil, err
	}
	return s.canonicalizeRankedRetainedResult(result, usePrem)
}
