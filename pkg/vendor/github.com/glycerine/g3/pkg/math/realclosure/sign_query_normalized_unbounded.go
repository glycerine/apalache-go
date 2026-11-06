package realclosure

func (s *realValueStore) rootCountUnboundedNormalized(p []*realValue) (int, error) {
	rows, err := s.normalizedQuerySequence(p, nil)
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

func (s *realValueStore) tarskiUnboundedNormalized(p, q []*realValue) (int, error) {
	rows, err := s.normalizedQuerySequence(p, q)
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
