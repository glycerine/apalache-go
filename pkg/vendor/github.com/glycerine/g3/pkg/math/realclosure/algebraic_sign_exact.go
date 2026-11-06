package realclosure

// Source: src/math/realclosure/realclosure.cpp:4460-4766, concrete
// single-root rational-polynomial algebraic extension.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) exactSingleRootPolynomialSign(value *realValue, usePrem bool) (int32, error) {
	if s == nil || value == nil || value.store != s || value.deleted || value.refs == 0 || value.isRational || value.root == nil {
		return 0, arena.ErrInvalidSlot
	}
	root := value.root
	if root.deleted || root.signs != nil || root.isolating == nil || len(root.polynomial.coefficients) < 2 {
		return 0, arena.ErrInvalidSlot
	}
	query, err := s.tarskiBinaryIntervalRational(root.polynomial.coefficients, value.functionNumerator.coefficients, root.isolating, usePrem)
	if err != nil {
		return 0, err
	}
	if query < 0 {
		return -1, nil
	}
	if query > 0 {
		return 1, nil
	}
	return 0, nil
}
