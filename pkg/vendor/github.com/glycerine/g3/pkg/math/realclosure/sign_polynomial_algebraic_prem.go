package realclosure

import "github.com/glycerine/g3/pkg/arena"

// pseudoRemainderNormalized is prem over the retained value field. It avoids
// coefficient division and returns the number of leading-coefficient steps.
func (s *realValueStore) pseudoRemainderNormalized(left, right []*realValue) ([]*realValue, int, error) {
	if s == nil || len(right) == 0 || right[len(right)-1] == nil {
		return nil, 0, arena.ErrInvalidSlot
	}
	for _, values := range [][]*realValue{left, right} {
		for _, value := range values {
			if !s.realOperand(value) {
				return nil, 0, arena.ErrInvalidSlot
			}
		}
	}
	if len(right) == 1 {
		return nil, 0, nil
	}
	result := append([]*realValue(nil), left...)
	for i, value := range result {
		if err := s.retain(value); err != nil {
			_ = s.releaseTemporaryCoefficients(result[:i])
			return nil, 0, err
		}
	}
	leading := right[len(right)-1]
	steps := 0
	for len(result) >= len(right) {
		steps++
		shift := len(result) - len(right)
		high := result[len(result)-1]
		for i := 0; i < len(result)-1; i++ {
			updated, err := s.multiplyNormalizedValues(result[i], leading)
			if err != nil {
				_ = s.releaseTemporaryCoefficients(result)
				return nil, steps, err
			}
			if err = s.release(result[i]); err != nil {
				_ = s.release(updated)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, steps, err
			}
			result[i] = updated
		}
		for i := 0; i < len(right)-1; i++ {
			product, err := s.multiplyNormalizedValues(high, right[i])
			if err != nil {
				_ = s.releaseTemporaryCoefficients(result)
				return nil, steps, err
			}
			index := i + shift
			updated, err := s.subtractValues(result[index], product)
			releaseProduct := s.release(product)
			if err != nil {
				_ = s.releaseTemporaryCoefficients(result)
				return nil, steps, err
			}
			if releaseProduct != nil {
				_ = s.release(updated)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, steps, releaseProduct
			}
			if err = s.release(result[index]); err != nil {
				_ = s.release(updated)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, steps, err
			}
			result[index] = updated
		}
		if err := s.release(result[len(result)-1]); err != nil {
			_ = s.releaseTemporaryCoefficients(result)
			return nil, steps, err
		}
		result = trimRealCoefficients(result[:len(result)-1])
	}
	return result, steps, nil
}
