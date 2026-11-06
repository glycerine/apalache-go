package realclosure

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) shouldNormalizeAlgebraic(root *algebraicRoot) (bool, error) {
	if !s.lazyAlgebraicNormalizationEnabled() || !s.inAuxValues {
		return true, nil
	}
	if root == nil || len(root.polynomial.coefficients) == 0 {
		return false, arena.ErrInvalidSlot
	}
	leading := root.polynomial.coefficients[len(root.polynomial.coefficients)-1]
	if !s.rationalOperand(leading) || leading == nil || !leading.isRational {
		return false, nil
	}
	monic, err := s.numbers.IsOne(&leading.rational)
	return monic, err
}

// reduceAlgebraicNumerator computes rem(numerator, root.polynomial), matching
// normalize_algebraic's eager branch. Retained roots currently have concrete
// rational defining coefficients, so no universal lower-extension adapter is
// introduced here.
func (s *realValueStore) reduceAlgebraicNumerator(root *algebraicRoot, numerator []*realValue) ([]*realValue, error) {
	if root == nil || root.deleted || root.owner == nil || root.owner.values != s {
		return nil, arena.ErrInvalidSlot
	}
	divisor := root.polynomial.coefficients
	if len(divisor) < 2 || divisor[len(divisor)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	normalize, err := s.shouldNormalizeAlgebraic(root)
	if err != nil {
		return nil, err
	}
	if !normalize {
		result := append([]*realValue(nil), numerator...)
		for i, coefficient := range result {
			if err := s.retain(coefficient); err != nil {
				_ = s.releaseTemporaryCoefficients(result[:i])
				return nil, err
			}
		}
		return result, nil
	}
	for _, coefficient := range divisor {
		if !s.rationalOperand(coefficient) {
			return nil, arena.ErrInvalidSlot
		}
	}
	result := append([]*realValue(nil), numerator...)
	for i, coefficient := range result {
		if !s.rationalOperand(coefficient) {
			_ = s.releaseTemporaryCoefficients(result[:i])
			return nil, arena.ErrInvalidSlot
		}
		if err := s.retain(coefficient); err != nil {
			_ = s.releaseTemporaryCoefficients(result[:i])
			return nil, err
		}
	}
	leading := divisor[len(divisor)-1]
	for len(result) >= len(divisor) {
		shift := len(result) - len(divisor)
		high := result[len(result)-1]
		ratio, err := s.divideValueByRational(high, leading)
		if err != nil {
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		for i := 0; i < len(divisor)-1; i++ {
			product, cause := s.multiplyValues(ratio, divisor[i])
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
	return result, nil
}
