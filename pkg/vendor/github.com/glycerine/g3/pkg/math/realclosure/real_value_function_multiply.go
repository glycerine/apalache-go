package realclosure

import "github.com/glycerine/g3/pkg/arena"

// multiplyNormalizedValues extends the dependency packet's rational scaling
// entry point with source normalize_algebraic for same-extension products.
func (s *realValueStore) multiplyNormalizedValues(a, b *realValue) (*realValue, error) {
	if !s.realOperand(a) || !s.realOperand(b) {
		return nil, arena.ErrInvalidSlot
	}
	if a != nil && b != nil && !a.isRational && !b.isRational {
		return s.multiplyAlgebraicValues(a, b)
	}
	return s.multiplyValues(a, b)
}
