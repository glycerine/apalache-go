package realclosure

import "github.com/glycerine/g3/pkg/arena"

// signedRemainderNormalized is rem followed by neg for retained value
// polynomials whose coefficients belong to one algebraic extension.
func (s *realValueStore) signedRemainderNormalized(left, right []*realValue) ([]*realValue, error) {
	if s == nil || len(right) == 0 || right[len(right)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, values := range [][]*realValue{left, right} {
		for _, value := range values {
			if !s.realOperand(value) {
				return nil, arena.ErrInvalidSlot
			}
		}
	}
	if len(right) == 1 {
		return nil, nil
	}
	result := append([]*realValue(nil), left...)
	for i, value := range result {
		if err := s.retain(value); err != nil {
			_ = s.releaseTemporaryCoefficients(result[:i])
			return nil, err
		}
	}
	for len(result) >= len(right) {
		shift := len(result) - len(right)
		ratio, err := s.divideNormalizedValues(result[len(result)-1], right[len(right)-1])
		if err != nil {
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		for i := 0; i < len(right)-1; i++ {
			product, cause := s.multiplyNormalizedValues(ratio, right[i])
			if cause != nil {
				_ = s.release(ratio)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, cause
			}
			index := i + shift
			updated, cause := s.subtractValues(result[index], product)
			releaseProduct := s.release(product)
			if cause != nil {
				_ = s.release(ratio)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, cause
			}
			if releaseProduct != nil {
				_ = s.release(updated)
				_ = s.release(ratio)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, releaseProduct
			}
			if cause = s.release(result[index]); cause != nil {
				_ = s.release(updated)
				_ = s.release(ratio)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, cause
			}
			result[index] = updated
		}
		if err = s.release(ratio); err != nil {
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		if err = s.release(result[len(result)-1]); err != nil {
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		result = trimRealCoefficients(result[:len(result)-1])
	}
	for i, value := range result {
		negative, err := s.negateValue(value)
		if err != nil {
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		if err = s.release(value); err != nil {
			_ = s.release(negative)
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		result[i] = negative
	}
	return trimRealCoefficients(result), nil
}
