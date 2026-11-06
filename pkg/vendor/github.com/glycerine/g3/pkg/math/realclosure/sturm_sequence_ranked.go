package realclosure

// Source: src/math/realclosure/realclosure.cpp:3637-3702.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) sturmSequenceRanked(first, second []*realValue, usePrem bool) ([]realPolynomial, error) {
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
		var remainder []*realValue
		var err error
		if usePrem {
			remainder, err = s.signedPseudoRemainderRanked(rows[len(rows)-2].coefficients, rows[len(rows)-1].coefficients, usePrem)
			// Pseudo-remainder is an optional denominator-avoidance route in the
			// source. A tower intermediate can require equality across sibling
			// extensions while it is still being canonicalized; use the source's
			// field-remainder route for that step and retain identical Sturm signs.
			if errors.Is(err, arena.ErrInvalidSlot) {
				remainder, err = s.signedRemainderRanked(rows[len(rows)-2].coefficients, rows[len(rows)-1].coefficients, false)
			}
		} else {
			remainder, err = s.signedRemainderRanked(rows[len(rows)-2].coefficients, rows[len(rows)-1].coefficients, usePrem)
		}
		if err != nil {
			_ = s.closePolynomialSequence(rows)
			return nil, err
		}
		if len(remainder) == 0 {
			return rows, nil
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
