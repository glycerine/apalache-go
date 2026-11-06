package realclosure

// Source: src/math/realclosure/realclosure.cpp:3149-3191.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) cleanTowerValueDenominator(value *realValue, usePrem bool) (numerator, denominator *realValue, err error) {
	if value == nil || value.isRational {
		return s.cleanRationalValueDenominator(value)
	}
	root := value.root
	if root == nil || !root.isRegistered() {
		return nil, nil, arena.ErrInvalidSlot
	}
	numCoefficients, numeratorDenominator, err := s.cleanTowerPolynomialDenominators(value.functionNumerator.coefficients, usePrem)
	if err != nil {
		return nil, nil, err
	}
	var implicitOne *realValue
	denCoefficients := value.functionDenominator.coefficients
	if len(denCoefficients) == 0 {
		implicitOne, err = s.newRational(1, 1)
		if err != nil {
			return nil, nil, errors.Join(err, s.releaseSturmCoefficients(numCoefficients), s.releaseSturmScratch(numeratorDenominator))
		}
		denCoefficients = []*realValue{implicitOne}
	}
	defer func() {
		err = errors.Join(err,
			s.releaseSturmCoefficients(numCoefficients),
			s.releaseSturmScratch(numeratorDenominator),
			s.releaseSturmScratch(implicitOne))
		if err != nil {
			err = errors.Join(err, s.releaseSturmScratch(numerator), s.releaseSturmScratch(denominator))
			numerator, denominator = nil, nil
		}
	}()
	denCoefficients, denominatorDenominator, err := s.cleanTowerPolynomialDenominators(denCoefficients, usePrem)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		err = errors.Join(err, s.releaseSturmCoefficients(denCoefficients), s.releaseSturmScratch(denominatorDenominator))
	}()
	numeratorPolynomial, err := s.publishRankedPolynomialValue(root, numCoefficients, usePrem)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(numeratorPolynomial)) }()
	denominatorPolynomial, err := s.publishRankedPolynomialValue(root, denCoefficients, usePrem)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(denominatorPolynomial)) }()
	numerator, err = s.multiplyRankedRetainedValues(numeratorPolynomial, denominatorDenominator, usePrem)
	if err != nil {
		return nil, nil, err
	}
	denominator, err = s.multiplyRankedRetainedValues(denominatorPolynomial, numeratorDenominator, usePrem)
	if err != nil {
		return nil, nil, err
	}
	return numerator, denominator, nil
}
