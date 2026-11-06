package realclosure

// Source: src/math/realclosure/realclosure.cpp:4460-4766.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) determineTowerRetainedValueSign(value *realValue, usePrem bool) (int32, error) {
	if value == nil || value.isRational {
		return s.signValue(value)
	}
	if value.root == nil || value.root.deleted || value.functionInterval == nil || value.root.signs != nil {
		return 0, arena.ErrInvalidSlot
	}
	contains, err := value.functionInterval.ContainsZero()
	if err != nil {
		return 0, err
	}
	if !contains {
		return s.signValue(value)
	}
	query, err := s.tarskiBinaryIntervalRanked(value.root.polynomial.coefficients, value.functionNumerator.coefficients, value.root.isolating, usePrem)
	if err != nil || query == 0 {
		return int32(query), err
	}
	sign := int32(1)
	if query < 0 {
		sign = -1
	}
	if err = restrictBinaryIntervalToSign(value.functionInterval, sign); err != nil {
		return 0, err
	}
	return sign, nil
}
