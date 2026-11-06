package realclosure

import "github.com/glycerine/g3/pkg/arena"

// multiplyNormalizedPolynomials is the source value-polynomial convolution
// after same-extension algebraic multiplication becomes dependency-ready.
func (s *realValueStore) multiplyNormalizedPolynomials(left, right []*realValue) ([]*realValue, error) {
	if s == nil || s.numbers == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, value := range append(append([]*realValue(nil), left...), right...) {
		if !s.realOperand(value) {
			return nil, arena.ErrInvalidSlot
		}
	}
	result := make([]*realValue, len(left)*len(right))
	if len(left) < len(right) {
		left, right = right, left
	}
	for i, a := range left {
		if err := s.checkpoint(); err != nil {
			_ = s.releaseTemporaryCoefficients(result)
			return nil, err
		}
		if a == nil {
			continue
		}
		for j, b := range right {
			product, err := s.multiplyNormalizedValues(a, b)
			if err != nil {
				_ = s.releaseTemporaryCoefficients(result)
				return nil, err
			}
			sum, err := s.addValues(result[i+j], product)
			if releaseErr := s.release(product); err == nil {
				err = releaseErr
			}
			if err != nil {
				_ = s.release(sum)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, err
			}
			if err = s.release(result[i+j]); err != nil {
				_ = s.release(sum)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, err
			}
			result[i+j] = sum
		}
	}
	return trimRealCoefficients(result), nil
}
