package realclosure

// Source: src/math/realclosure/realclosure.cpp:3460-3495.

func (s *realValueStore) makeAnyPolynomialMonic(p []*realValue, usePrem bool) ([]*realValue, error) {
	if err := s.checkAnyPolynomial(p, true); err != nil {
		return nil, err
	}
	if len(p) == 0 {
		return nil, nil
	}
	return s.divideAnyPolynomialByScalar(p, p[len(p)-1], usePrem)
}
