package realclosure

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) multiplyRankedPolynomialByValue(coefficients []*realValue, value *realValue, usePrem bool) (result []*realValue, err error) {
	if len(coefficients) == 0 || value == nil {
		return nil, nil
	}
	result = make([]*realValue, len(coefficients))
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i, coefficient := range coefficients {
		result[i], err = s.multiplyRankedRetainedValues(coefficient, value, usePrem)
		if err != nil {
			return result, err
		}
	}
	return trimAnyPolynomial(result), nil
}

// normalizeRankedFraction mirrors normalize_fraction in realclosure.cpp:
// make the denominator monic and cancel the polynomial gcd.
func (s *realValueStore) normalizeRankedFraction(numerator, denominator []*realValue, usePrem bool) (normalizedNumerator, normalizedDenominator []*realValue, err error) {
	if len(numerator) == 0 || len(denominator) == 0 {
		return nil, nil, arena.ErrInvalidSlot
	}
	if len(denominator) == 1 {
		normalizedNumerator, err = s.divideRankedPolynomialByScalar(numerator, denominator[0], usePrem)
		if err != nil {
			return nil, nil, err
		}
		one, e := s.newRational(1, 1)
		if e != nil {
			return nil, nil, errors.Join(e, s.releaseSturmCoefficients(normalizedNumerator))
		}
		return normalizedNumerator, []*realValue{one}, nil
	}
	leading := denominator[len(denominator)-1]
	monicNumerator, err := s.divideRankedPolynomialByScalar(numerator, leading, usePrem)
	if err != nil {
		return nil, nil, err
	}
	monicDenominator, err := s.makeRankedPolynomialMonic(denominator, usePrem)
	if err != nil {
		return nil, nil, errors.Join(err, s.releaseSturmCoefficients(monicNumerator))
	}
	defer func() {
		err = errors.Join(err, s.releaseSturmCoefficients(monicNumerator), s.releaseSturmCoefficients(monicDenominator))
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(normalizedNumerator), s.releaseSturmCoefficients(normalizedDenominator))
			normalizedNumerator, normalizedDenominator = nil, nil
		}
	}()
	gcd, err := s.gcdRankedPolynomials(monicNumerator, monicDenominator, usePrem)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(gcd)) }()
	if s.rankedDenominatorIsOne(gcd) {
		normalizedNumerator, err = s.retainRationalPolynomial(monicNumerator)
		if err != nil {
			return nil, nil, err
		}
		normalizedDenominator, err = s.retainRationalPolynomial(monicDenominator)
		return normalizedNumerator, normalizedDenominator, err
	}
	var numeratorRemainder, denominatorRemainder []*realValue
	normalizedNumerator, numeratorRemainder, err = s.divideRankedPolynomials(monicNumerator, gcd, usePrem)
	if err != nil {
		return nil, nil, err
	}
	if len(numeratorRemainder) != 0 {
		return nil, nil, errors.Join(arena.ErrInvalidSlot, s.releaseSturmCoefficients(numeratorRemainder))
	}
	normalizedDenominator, denominatorRemainder, err = s.divideRankedPolynomials(monicDenominator, gcd, usePrem)
	if err != nil {
		return nil, nil, err
	}
	if len(denominatorRemainder) != 0 {
		return nil, nil, errors.Join(arena.ErrInvalidSlot, s.releaseSturmCoefficients(denominatorRemainder))
	}
	return normalizedNumerator, normalizedDenominator, nil
}
