package realclosure

// Source: src/math/realclosure/realclosure.cpp:1588-1611.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) hasPendingSturmTemporaries() bool {
	return len(s.pendingSturmCoefficients) != 0 || len(s.pendingSturmRationals) != 0 || len(s.pendingSturmIntegers) != 0 || len(s.pendingSturmBinaries) != 0 || len(s.pendingSturmIntervals) != 0
}

// closePolynomialSequenceAfterTemporaries preserves C++ lexical destruction:
// refused coefficient/numeric temporaries are younger than the polynomial
// sequence and must drain before its rows.
func (s *realValueStore) closePolynomialSequenceAfterTemporaries(rows []realPolynomial) error {
	if len(rows) == 0 {
		return nil
	}
	if s.hasPendingSturmTemporaries() {
		if s.pendingSequence != nil {
			return errRealClosureSequencePending
		}
		s.pendingSequence = rows
		return nil
	}
	return s.closePolynomialSequence(rows)
}

// derivativeSequenceRational implements mk_derivatives for the retained
// rational coefficient instantiation. The source stores p', p”, ... through
// the final constant derivative; sign determination scans this sequence in
// reverse order.
func (s *realValueStore) derivativeSequenceRational(p []*realValue) (rows []realPolynomial, err error) {
	if err = s.sequenceReady(); err != nil {
		return nil, err
	}
	if len(p) < 3 || p[0] == nil || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, value := range p {
		if !s.rationalOperand(value) {
			return nil, arena.ErrInvalidSlot
		}
	}
	current := p
	for i := 0; i < len(p)-1; i++ {
		coefficients, bodyErr := s.derivativeRationalPolynomial(current)
		if bodyErr != nil {
			cleanupErr := s.closePolynomialSequenceAfterTemporaries(rows)
			return nil, errors.Join(bodyErr, cleanupErr)
		}
		var row realPolynomial
		bodyErr = s.setPolynomial(&row, coefficients)
		if bodyErr == nil {
			rows = append(rows, row)
			current = rows[len(rows)-1].coefficients
		}
		releaseErr := s.releaseSturmCoefficients(coefficients)
		if bodyErr != nil || releaseErr != nil {
			cleanupErr := s.closePolynomialSequenceAfterTemporaries(rows)
			return nil, errors.Join(bodyErr, releaseErr, cleanupErr)
		}
	}
	return rows, nil
}

func (s *realValueStore) derivativeSequenceAny(p []*realValue, usePrem bool) (rows []realPolynomial, err error) {
	if err = s.sequenceReady(); err != nil {
		return nil, err
	}
	if len(p) < 3 || p[0] == nil || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err = s.checkAnyPolynomial(p, false); err != nil {
		return nil, err
	}
	current := p
	for i := 0; i < len(p)-1; i++ {
		coefficients, bodyErr := s.derivativeAnyPolynomial(current, usePrem)
		if bodyErr != nil {
			cleanupErr := s.closePolynomialSequenceAfterTemporaries(rows)
			return nil, errors.Join(bodyErr, cleanupErr)
		}
		var row realPolynomial
		bodyErr = s.setPolynomial(&row, coefficients)
		if bodyErr == nil {
			rows = append(rows, row)
			current = rows[len(rows)-1].coefficients
		}
		releaseErr := s.releaseSturmCoefficients(coefficients)
		if bodyErr != nil || releaseErr != nil {
			cleanupErr := s.closePolynomialSequenceAfterTemporaries(rows)
			return nil, errors.Join(bodyErr, releaseErr, cleanupErr)
		}
	}
	return rows, nil
}
