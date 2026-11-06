package realclosure

// Source: src/math/realclosure/realclosure.cpp:4867-5085, 5684-5720.

import (
	"errors"
	"fmt"
)

func (s *realValueStore) subtractTowerValuesUncanonicalized(left, right *realValue, usePrem bool) (result *realValue, err error) {
	negative, err := s.negateRankedRetainedValue(right, usePrem)
	if err != nil {
		return nil, fmt.Errorf("tower subtract negate: %w", err)
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(negative)) }()
	comparison, err := s.compareRetainedValueRanks(left, negative)
	if err != nil {
		return nil, fmt.Errorf("tower subtract rank: %w", err)
	}
	if comparison == 0 {
		result, err := s.addTowerCanonicalValues(left, negative, usePrem)
		if err != nil {
			return result, fmt.Errorf("tower subtract equal-rank add: %w", err)
		}
		return result, nil
	}
	higher, lower := left, negative
	if comparison < 0 {
		higher, lower = negative, left
	}
	numerator, err := s.retainRationalPolynomial(higher.functionNumerator.coefficients)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(numerator)) }()
	updated, err := s.addTowerCanonicalValues(numerator[0], lower, usePrem)
	if err != nil {
		return nil, fmt.Errorf("tower subtract constant add: %w", err)
	}
	if err = s.releaseSturmScratch(numerator[0]); err != nil {
		_ = s.releaseSturmScratch(updated)
		return nil, err
	}
	numerator[0] = updated
	approximation, err := NewBinaryInterval(higher.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	leftInterval, err := s.valueInterval(left, higher.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	rightInterval, err := s.valueInterval(right, higher.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	negativeInterval, err := NewBinaryInterval(higher.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, negativeInterval.Close()) }()
	if err = negativeInterval.Neg(rightInterval); err != nil {
		return nil, err
	}
	if err = approximation.Add(leftInterval, negativeInterval); err != nil {
		return nil, err
	}
	return s.newRankedAlgebraicPolynomialValue(higher.root, numerator, approximation)
}

func (s *realValueStore) addTowerValuesUncanonicalized(left, right *realValue, usePrem bool) (result *realValue, err error) {
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
	leftCoefficients := higher.functionNumerator.coefficients
	rightCoefficients := []*realValue{lower}
	if comparison == 0 {
		leftCoefficients = left.functionNumerator.coefficients
		rightCoefficients = right.functionNumerator.coefficients
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
		numerator[i], err = s.addTowerCanonicalValues(a, b, usePrem)
		if err != nil {
			return nil, err
		}
	}
	numerator = trimAnyPolynomial(numerator)
	if len(numerator) == 0 {
		return nil, nil
	}
	if len(numerator) == 1 {
		result = numerator[0]
		numerator[0] = nil
		return result, nil
	}
	approximation, err := NewBinaryInterval(higher.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	leftInterval, err := s.valueInterval(left, higher.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	rightInterval, err := s.valueInterval(right, higher.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	if err = approximation.Add(leftInterval, rightInterval); err != nil {
		return nil, err
	}
	return s.newRankedAlgebraicPolynomialValue(higher.root, numerator, approximation)
}

func (s *realValueStore) addTowerCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	result, err := s.addTowerValuesUncanonicalized(left, right, usePrem)
	if err != nil || result == nil || result.isRational {
		return result, err
	}
	sign, err := s.determineTowerRetainedValueSign(result, usePrem)
	if err != nil {
		return result, err
	}
	if sign != 0 {
		return result, nil
	}
	if err := s.release(result); err != nil {
		return result, err
	}
	return nil, nil
}

func (s *realValueStore) compareTowerRetainedValues(left, right *realValue, usePrem bool) (comparison int, err error) {
	if left == right {
		return 0, nil
	}
	difference, err := s.subtractTowerValuesUncanonicalized(left, right, usePrem)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(difference)) }()
	sign, err := s.determineTowerRetainedValueSign(difference, usePrem)
	return int(sign), err
}

func (s *realValueStore) subtractTowerCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	result, err := s.subtractTowerValuesUncanonicalized(left, right, usePrem)
	if err != nil || result == nil || result.isRational {
		return result, err
	}
	sign, err := s.determineTowerRetainedValueSign(result, usePrem)
	if err != nil {
		return result, err
	}
	if sign != 0 {
		return result, nil
	}
	if err := s.release(result); err != nil {
		return result, err
	}
	return nil, nil
}
