package realclosure

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) rootCountIntervalNormalized(p []*realValue, interval *BinaryInterval) (int, error) {
	if interval == nil {
		return 0, arena.ErrInvalidSlot
	}
	if err := interval.open(); err != nil {
		return 0, err
	}
	rows, err := s.normalizedQuerySequence(p, nil)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	count, err := s.tarskiFromBinaryIntervalSequence(rows, interval)
	closeErr := s.closePolynomialSequence(rows)
	if err != nil {
		return 0, err
	}
	return count, closeErr
}

func (s *realValueStore) tarskiIntervalNormalized(p, q []*realValue, interval *BinaryInterval) (int, error) {
	if interval == nil {
		return 0, arena.ErrInvalidSlot
	}
	if err := interval.open(); err != nil {
		return 0, err
	}
	rows, err := s.normalizedQuerySequence(p, q)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	query, err := s.tarskiFromBinaryIntervalSequence(rows, interval)
	closeErr := s.closePolynomialSequence(rows)
	if err != nil {
		return 0, err
	}
	return query, closeErr
}
