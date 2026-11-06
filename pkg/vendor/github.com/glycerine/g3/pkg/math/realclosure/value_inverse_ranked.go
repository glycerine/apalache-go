package realclosure

// Source: src/math/realclosure/realclosure.cpp:5330-5585.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) inverseRankedRetainedValue(value *realValue, usePrem bool) (result *realValue, err error) {
	if value == nil || value.store != s || value.deleted || value.refs == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.inverseRetainedValue(value)
	}
	root := value.root
	if root == nil || !root.isRegistered() {
		return nil, arena.ErrInvalidSlot
	}
	if root.kind != realExtensionAlgebraic {
		if len(value.functionDenominator.coefficients) == 0 {
			return nil, arena.ErrInvalidSlot
		}
		approximation, e := NewBinaryInterval(root.owner.intervals)
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, approximation.Close()) }()
		if err = approximation.Inv(value.functionInterval, s.rationalInitialPrecision()); err != nil {
			return nil, err
		}
		numerator, denominator, e := s.normalizeRankedFraction(value.functionDenominator.coefficients, value.functionNumerator.coefficients, usePrem)
		if e != nil {
			return nil, e
		}
		defer func() {
			err = errors.Join(err, s.releaseSturmCoefficients(numerator), s.releaseSturmCoefficients(denominator))
		}()
		result, err = s.newRankedRationalFunctionValue(root, numerator, denominator, approximation)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	normalized, err := s.reduceRankedAlgebraicPolynomial(root, value.functionNumerator.coefficients, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(normalized)) }()
	inverse, coprime, err := s.invertRankedPolynomialModulo(normalized, root.polynomial.coefficients, usePrem)
	if err != nil {
		return nil, err
	}
	if !coprime {
		return nil, arena.ErrInvalidSlot
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(inverse)) }()
	if len(inverse) == 1 {
		result = inverse[0]
		inverse[0] = nil
		return result, nil
	}
	approximation, err := NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	if err = approximation.Inv(value.functionInterval, s.rationalInitialPrecision()); err != nil {
		return nil, err
	}
	result, err = s.newRankedAlgebraicPolynomialValue(root, inverse, approximation)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeRankedRetainedResult(result, usePrem)
}
