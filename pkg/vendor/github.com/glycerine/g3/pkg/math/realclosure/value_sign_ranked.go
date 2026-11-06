package realclosure

// Source: src/math/realclosure/realclosure.cpp:4460-4766.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) determineRankedRetainedValueSign(value *realValue, usePrem bool) (int32, error) {
	if value == nil || value.isRational {
		return s.signValue(value)
	}
	if value.root == nil || value.root.deleted || value.functionInterval == nil {
		return 0, arena.ErrInvalidSlot
	}
	contains, err := value.functionInterval.ContainsZero()
	if err != nil {
		return 0, err
	}
	if !contains {
		return s.signValue(value)
	}
	if value.root.kind != realExtensionAlgebraic && len(value.functionDenominator.coefficients) != 0 && !s.rankedDenominatorIsOne(value.functionDenominator.coefficients) {
		numeratorSign, err := s.determineRankedPolynomialSign(value.root, value.functionNumerator.coefficients, usePrem)
		if err != nil || numeratorSign == 0 {
			return numeratorSign, err
		}
		denominatorSign, err := s.determineRankedPolynomialSign(value.root, value.functionDenominator.coefficients, usePrem)
		if err != nil || denominatorSign == 0 {
			return 0, err
		}
		sign := numeratorSign * denominatorSign
		if err = restrictBinaryIntervalToSign(value.functionInterval, sign); err != nil {
			return 0, err
		}
		return sign, nil
	}
	if value.root.kind == realExtensionTranscendental {
		for precision := uint32(1); ; precision++ {
			if _, err := s.refineTranscendentalPolynomialValue(value, precision); err != nil {
				return 0, err
			}
			contains, err = value.functionInterval.ContainsZero()
			if err != nil {
				return 0, err
			}
			if !contains {
				return s.signValue(value)
			}
			if precision == ^uint32(0) {
				return 0, arena.ErrBounds
			}
		}
	}
	if value.root.kind == realExtensionInfinitesimal {
		return s.determineInfinitesimalValueSign(value, usePrem)
	}
	if value.root.signs != nil {
		return s.determineAnyRetainedValueSign(value, usePrem)
	}
	sign, err := s.tarskiBinaryIntervalAny(value.root.polynomial.coefficients, value.functionNumerator.coefficients, value.root.isolating, usePrem)
	if err != nil || sign == 0 {
		return int32(sign), err
	}
	result := int32(1)
	if sign < 0 {
		result = -1
	}
	if err = restrictBinaryIntervalToSign(value.functionInterval, result); err != nil {
		return 0, err
	}
	return result, nil
}

func (s *realValueStore) determineRankedPolynomialSign(root *algebraicRoot, coefficients []*realValue, usePrem bool) (sign int32, err error) {
	value, err := s.publishRankedPolynomialValue(root, coefficients, usePrem)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(value)) }()
	return s.determineRankedRetainedValueSign(value, usePrem)
}
