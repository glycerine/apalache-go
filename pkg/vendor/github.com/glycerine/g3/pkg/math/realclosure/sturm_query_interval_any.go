package realclosure

// Source: src/math/realclosure/realclosure.cpp:3930-4069.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) rootCountBinaryIntervalAny(p []*realValue, interval *BinaryInterval, usePrem bool) (int, error) {
	if interval == nil {
		return 0, arena.ErrInvalidSlot
	}
	if err := interval.open(); err != nil {
		return 0, err
	}
	rows, err := s.rootSturmSequenceAny(p, usePrem)
	if err != nil {
		return 0, err
	}
	count, queryErr := s.tarskiFromBinaryIntervalSequenceAny(rows, interval, usePrem)
	closeErr := s.closePolynomialSequence(rows)
	if queryErr != nil {
		return 0, queryErr
	}
	return count, closeErr
}

func (s *realValueStore) tarskiBinaryIntervalAny(p, q []*realValue, interval *BinaryInterval, usePrem bool) (int, error) {
	if interval == nil {
		return 0, arena.ErrInvalidSlot
	}
	if err := interval.open(); err != nil {
		return 0, err
	}
	rows, err := s.tarskiSturmSequenceAny(p, q, usePrem)
	if err != nil {
		return 0, err
	}
	query, queryErr := s.tarskiFromBinaryIntervalSequenceAny(rows, interval, usePrem)
	closeErr := s.closePolynomialSequence(rows)
	if queryErr != nil {
		return 0, queryErr
	}
	return query, closeErr
}
