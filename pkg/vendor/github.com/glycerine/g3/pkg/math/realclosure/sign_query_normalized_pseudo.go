package realclosure

func (s *realValueStore) rootCountUnboundedPseudoNormalized(p []*realValue) (int, error) {
	rows, err := s.normalizedPseudoQuerySequence(p, nil)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	count, err := s.tarskiFromUnboundedSequence(rows)
	closeErr := s.closePolynomialSequence(rows)
	if err != nil {
		return 0, err
	}
	return count, closeErr
}

func (s *realValueStore) tarskiUnboundedPseudoNormalized(p, q []*realValue) (int, error) {
	rows, err := s.normalizedPseudoQuerySequence(p, q)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	query, err := s.tarskiFromUnboundedSequence(rows)
	closeErr := s.closePolynomialSequence(rows)
	if err != nil {
		return 0, err
	}
	return query, closeErr
}
