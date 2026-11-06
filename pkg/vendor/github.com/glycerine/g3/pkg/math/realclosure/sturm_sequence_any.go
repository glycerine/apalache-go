package realclosure

// Source: src/math/realclosure/realclosure.cpp:3637-3702.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) sturmSequenceAny(first, second []*realValue, usePrem bool) ([]realPolynomial, error) {
	if err := s.sequenceReady(); err != nil {
		return nil, err
	}
	restoreAuxiliary := s.withAuxiliaryValues()
	defer restoreAuxiliary()
	if err := s.checkAnyPolynomial(first, false); err != nil {
		return nil, err
	}
	if err := s.checkAnyPolynomial(second, false); err != nil {
		return nil, err
	}
	rows := make([]realPolynomial, 2)
	if err := s.setPolynomial(&rows[0], first); err != nil {
		return nil, err
	}
	if err := s.setPolynomial(&rows[1], second); err != nil {
		_ = s.closePolynomialSequence(rows)
		return nil, err
	}
	for {
		previous := rows[len(rows)-2].coefficients
		current := rows[len(rows)-1].coefficients
		var remainder []*realValue
		var err error
		if usePrem {
			remainder, err = s.signedPseudoRemainderAny(previous, current, usePrem)
			if err == nil && len(remainder) > 0 {
				remainder, err = s.normalizeSequencePolynomialAny(remainder, usePrem)
			}
		} else {
			remainder, err = s.signedRemainderAny(previous, current, usePrem)
		}
		if err != nil {
			_ = s.closePolynomialSequence(rows)
			return nil, err
		}
		if len(remainder) == 0 {
			return rows, nil
		}
		if remainder[len(remainder)-1] == nil {
			_ = s.releaseSturmCoefficients(remainder)
			_ = s.closePolynomialSequence(rows)
			return nil, arena.ErrInvalidSlot
		}
		var next realPolynomial
		if err = s.setPolynomial(&next, remainder); err != nil {
			_ = s.releaseSturmCoefficients(remainder)
			_ = s.closePolynomialSequence(rows)
			return nil, err
		}
		if err = s.releaseSturmCoefficients(remainder); err != nil {
			_ = s.resetPolynomial(&next)
			_ = s.closePolynomialSequence(rows)
			return nil, err
		}
		rows = append(rows, next)
	}
}
