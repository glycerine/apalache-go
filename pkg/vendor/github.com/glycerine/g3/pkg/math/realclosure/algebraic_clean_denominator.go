package realclosure

// Source: src/math/realclosure/realclosure.cpp:3090-3165, concrete retained
// denominator-one algebraic polynomial value.

import "errors"

func (s *realValueStore) cleanAnyValueDenominator(value *realValue, usePrem bool) (numerator, denominator *realValue, err error) {
	if value == nil || value.isRational {
		return s.cleanRationalValueDenominator(value)
	}
	normalized, denominator, err := s.cleanRationalPolynomialDenominators(value.functionNumerator.coefficients)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		err = errors.Join(err, s.releaseSturmCoefficients(normalized))
		if err != nil {
			err = errors.Join(err, s.releaseSturmScratch(numerator), s.releaseSturmScratch(denominator))
			numerator, denominator = nil, nil
		}
	}()
	denominatorInterval, err := s.valueInterval(denominator, value.root.owner.intervals)
	if err != nil {
		return nil, nil, err
	}
	approximation, err := NewBinaryInterval(value.root.owner.intervals)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	if err = approximation.Mul(value.functionInterval, denominatorInterval); err != nil {
		return nil, nil, err
	}
	numerator, err = s.publishAlgebraicPolynomialResult(value.root, normalized, approximation)
	if err != nil {
		return nil, nil, err
	}
	numerator, err = s.canonicalizeAnyRetainedResult(numerator, usePrem)
	return numerator, denominator, err
}
