package realclosure

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) divideNormalizedValues(a, b *realValue) (*realValue, error) {
	if !s.realOperand(a) || !s.realOperand(b) || b == nil {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil {
		return nil, nil
	}
	if a.isRational && b.isRational {
		return s.divideRationals(a, b)
	}
	inverse, err := s.inverseNormalizedValue(b)
	if err != nil {
		return nil, err
	}
	out, err := s.multiplyNormalizedValues(a, inverse)
	releaseErr := s.release(inverse)
	if err != nil {
		return nil, err
	}
	if releaseErr != nil {
		_ = s.release(out)
		return nil, releaseErr
	}
	return out, nil
}
