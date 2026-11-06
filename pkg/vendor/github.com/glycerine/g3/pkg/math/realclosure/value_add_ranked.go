package realclosure

// Source: src/math/realclosure/realclosure.cpp:4867-5055.

import (
	"errors"
	"fmt"
)

func (s *realValueStore) addRankedRetainedValues(left, right *realValue, usePrem bool) (result *realValue, err error) {
	comparison, err := s.compareRetainedValueRanks(left, right)
	if err != nil {
		return nil, err
	}
	if left == nil {
		if err = s.retain(right); err != nil {
			return nil, err
		}
		return right, nil
	}
	if right == nil {
		if err = s.retain(left); err != nil {
			return nil, err
		}
		return left, nil
	}
	if left.isRational && right.isRational {
		return s.addRationals(left, right)
	}
	higher, lower := left, right
	if comparison < 0 {
		higher, lower = right, left
	}
	root := higher.root
	var denominator []*realValue
	var ownedDenominator bool
	var combinedNumerator []*realValue
	leftCoefficients := higher.functionNumerator.coefficients
	rightCoefficients := []*realValue{lower}
	if root.kind != realExtensionAlgebraic {
		leftDenominator := higher.functionDenominator.coefficients
		rightDenominator := leftDenominator
		if comparison == 0 {
			leftDenominator = left.functionDenominator.coefficients
			rightDenominator = right.functionDenominator.coefficients
		}
		if !s.rankedDenominatorIsOne(leftDenominator) || !s.rankedDenominatorIsOne(rightDenominator) {
			if comparison == 0 {
				leftScaled, e := s.multiplyRankedPolynomials(left.functionNumerator.coefficients, rightDenominator, usePrem)
				if e != nil {
					return nil, e
				}
				rightScaled, e := s.multiplyRankedPolynomials(right.functionNumerator.coefficients, leftDenominator, usePrem)
				if e != nil {
					return nil, errors.Join(e, s.releaseSturmCoefficients(leftScaled))
				}
				combinedNumerator, e = s.addRankedPolynomials(leftScaled, rightScaled, usePrem)
				e = errors.Join(e, s.releaseSturmCoefficients(leftScaled), s.releaseSturmCoefficients(rightScaled))
				if e != nil {
					return nil, e
				}
				denominator, e = s.multiplyRankedPolynomials(leftDenominator, rightDenominator, usePrem)
				if e != nil {
					return nil, errors.Join(e, s.releaseSturmCoefficients(combinedNumerator))
				}
				ownedDenominator = true
				leftCoefficients = combinedNumerator
			} else {
				scaled, e := s.multiplyRankedPolynomialByValue(leftDenominator, lower, usePrem)
				if e != nil {
					return nil, e
				}
				combinedNumerator, e = s.addRankedPolynomials(higher.functionNumerator.coefficients, scaled, usePrem)
				e = errors.Join(e, s.releaseSturmCoefficients(scaled))
				if e != nil {
					return nil, e
				}
				leftCoefficients = combinedNumerator
				denominator = leftDenominator
			}
		} else if comparison == 0 {
			leftCoefficients = left.functionNumerator.coefficients
			rightCoefficients = right.functionNumerator.coefficients
		}
	} else if comparison == 0 {
		leftCoefficients = left.functionNumerator.coefficients
		rightCoefficients = right.functionNumerator.coefficients
	}
	var numerator []*realValue
	if denominator == nil {
		size := len(leftCoefficients)
		if len(rightCoefficients) > size {
			size = len(rightCoefficients)
		}
		numerator = make([]*realValue, size)
		defer func() { err = errors.Join(err, s.releaseSturmCoefficients(numerator)) }()
		for i := 0; i < size; i++ {
			var a, b *realValue
			if i < len(leftCoefficients) {
				a = leftCoefficients[i]
			}
			if i < len(rightCoefficients) {
				b = rightCoefficients[i]
			}
			numerator[i], err = s.addRankedRetainedValues(a, b, usePrem)
			if err != nil {
				return nil, err
			}
		}
		numerator = trimAnyPolynomial(numerator)
	} else {
		numerator = combinedNumerator
		defer func() { err = errors.Join(err, s.releaseSturmCoefficients(numerator)) }()
		if ownedDenominator {
			defer func() { err = errors.Join(err, s.releaseSturmCoefficients(denominator)) }()
		}
	}
	if len(numerator) == 0 {
		return nil, nil
	}
	if len(numerator) == 1 && (root.kind == realExtensionAlgebraic || denominator == nil || s.rankedDenominatorIsOne(denominator)) {
		result = numerator[0]
		numerator[0] = nil
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
	if err = approximation.Add(leftInterval, rightInterval); err != nil {
		return nil, err
	}
	if root.kind == realExtensionAlgebraic {
		result, err = s.newRankedAlgebraicPolynomialValue(root, numerator, approximation)
	} else {
		if denominator == nil {
			denominator = higher.functionDenominator.coefficients
		}
		var normalizedNumerator, normalizedDenominator []*realValue
		normalizedNumerator, normalizedDenominator, err = s.normalizeRankedFraction(numerator, denominator, usePrem)
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
	result, err = s.canonicalizeRankedRetainedResult(result, usePrem)
	if err != nil {
		return result, fmt.Errorf("realclosure: canonicalize extension sum: %w", err)
	}
	return result, nil
}
