package realclosure

// Source: src/math/realclosure/realclosure.cpp:3930-4069.

func (s *realValueStore) rootCountUnboundedAny(p []*realValue, usePrem bool) (int, error) {
	rows, err := s.rootSturmSequenceAny(p, usePrem)
	if err != nil {
		return 0, err
	}
	count, queryErr := s.tarskiFromUnboundedSequenceAny(rows, usePrem)
	closeErr := s.closePolynomialSequence(rows)
	if queryErr != nil {
		return 0, queryErr
	}
	return count, closeErr
}

func (s *realValueStore) tarskiUnboundedAny(p, q []*realValue, usePrem bool) (int, error) {
	rows, err := s.tarskiSturmSequenceAny(p, q, usePrem)
	if err != nil {
		return 0, err
	}
	query, queryErr := s.tarskiFromUnboundedSequenceAny(rows, usePrem)
	closeErr := s.closePolynomialSequence(rows)
	if queryErr != nil {
		return 0, queryErr
	}
	return query, closeErr
}
