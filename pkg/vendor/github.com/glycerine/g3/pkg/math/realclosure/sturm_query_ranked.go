package realclosure

// Source: src/math/realclosure/realclosure.cpp:3999-4069.

func (s *realValueStore) tarskiBinaryIntervalRanked(p, q []*realValue, interval *BinaryInterval, usePrem bool) (int, error) {
	rows, err := s.tarskiSturmSequenceRanked(p, q, usePrem)
	if err != nil {
		return 0, err
	}
	query, queryErr := s.tarskiFromBinaryIntervalSequenceRanked(rows, interval, usePrem)
	closeErr := s.closePolynomialSequence(rows)
	if queryErr != nil {
		return 0, queryErr
	}
	return query, closeErr
}

func (s *realValueStore) rootCountBinaryIntervalRanked(p []*realValue, interval *BinaryInterval, usePrem bool) (int, error) {
	rows, err := s.rootSturmSequenceRanked(p, usePrem)
	if err != nil {
		return 0, err
	}
	count, queryErr := s.tarskiFromBinaryIntervalSequenceRanked(rows, interval, usePrem)
	closeErr := s.closePolynomialSequence(rows)
	if queryErr != nil {
		return 0, queryErr
	}
	return count, closeErr
}
