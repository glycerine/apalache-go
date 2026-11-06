package realclosure

// Source: src/math/realclosure/realclosure.cpp:3590-3606.

import "github.com/glycerine/g3/pkg/arena"

// derivativeRationalPolynomial follows the source coefficient loop and
// trailing-zero adjustment. Returned coefficients own one reference each.
func (s *realValueStore) derivativeRationalPolynomial(coefficients []*realValue) ([]*realValue, error) {
	if s == nil || s.numbers == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, value := range coefficients {
		if !s.rationalOperand(value) {
			return nil, arena.ErrInvalidSlot
		}
	}
	if len(coefficients) <= 1 {
		return nil, nil
	}
	result := make([]*realValue, 0, len(coefficients)-1)
	for i := 1; i < len(coefficients); i++ {
		scalar, err := s.newRational(int64(i), 1)
		if err != nil {
			_ = s.releaseSturmCoefficients(result)
			return nil, err
		}
		product, err := s.multiplyRationals(scalar, coefficients[i])
		if releaseErr := s.releaseSturmScratch(scalar); err == nil {
			err = releaseErr
		}
		if err != nil {
			_ = s.releaseSturmScratch(product)
			_ = s.releaseSturmCoefficients(result)
			return nil, err
		}
		result = append(result, product)
	}
	for len(result) > 0 && result[len(result)-1] == nil {
		result = result[:len(result)-1]
	}
	return result, nil
}

// derivativePolynomial is the source value-polynomial derivative. It keeps
// algebraic coefficient ownership and uses only rational integer scaling.
func (s *realValueStore) derivativePolynomial(coefficients []*realValue) ([]*realValue, error) {
	if s == nil || s.numbers == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, value := range coefficients {
		if !s.realOperand(value) {
			return nil, arena.ErrInvalidSlot
		}
	}
	if len(coefficients) <= 1 {
		return nil, nil
	}
	result := make([]*realValue, 0, len(coefficients)-1)
	for i := 1; i < len(coefficients); i++ {
		scalar, err := s.newRational(int64(i), 1)
		if err != nil {
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		product, err := s.multiplyValues(scalar, coefficients[i])
		if releaseErr := s.release(scalar); err == nil {
			err = releaseErr
		}
		if err != nil {
			_ = s.release(product)
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		result = append(result, product)
	}
	return trimRealCoefficients(result), nil
}
