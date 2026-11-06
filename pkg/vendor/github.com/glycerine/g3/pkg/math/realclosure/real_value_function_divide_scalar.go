package realclosure

import "github.com/glycerine/g3/pkg/arena"

// divideValueByRational is the concrete coefficient division used by
// polynomial remainder normalization for retained rational defining polynomials.
func (s *realValueStore) divideValueByRational(a, b *realValue) (*realValue, error) {
	if !s.realOperand(a) || !s.rationalOperand(b) || b == nil {
		return nil, arena.ErrInvalidSlot
	}
	if a == nil || a.isRational {
		return s.divideRationals(a, b)
	}
	one, err := s.newRational(1, 1)
	if err != nil {
		return nil, err
	}
	inverse, err := s.divideRationals(one, b)
	releaseErr := s.release(one)
	if err != nil {
		return nil, err
	}
	if releaseErr != nil {
		_ = s.release(inverse)
		return nil, releaseErr
	}
	out, err := s.multiplyValues(a, inverse)
	releaseErr = s.release(inverse)
	if err != nil {
		return nil, err
	}
	if releaseErr != nil {
		_ = s.release(out)
		return nil, releaseErr
	}
	return out, nil
}
