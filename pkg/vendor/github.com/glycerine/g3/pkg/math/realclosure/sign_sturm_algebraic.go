package realclosure

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) sturmSequenceNormalized(first, second []*realValue) ([]realPolynomial, error) {
	if err := s.sequenceReady(); err != nil {
		return nil, err
	}
	restoreAuxiliary := s.withAuxiliaryValues()
	defer restoreAuxiliary()
	if len(first) == 0 || len(second) == 0 {
		return nil, arena.ErrInvalidSlot
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
		remainder, err := s.signedRemainderNormalized(rows[len(rows)-2].coefficients, rows[len(rows)-1].coefficients)
		if err != nil {
			_ = s.closePolynomialSequence(rows)
			return nil, err
		}
		if len(remainder) == 0 {
			return rows, nil
		}
		var next realPolynomial
		if err = s.setPolynomial(&next, remainder); err != nil {
			_ = s.releaseTemporaryCoefficients(remainder)
			_ = s.closePolynomialSequence(rows)
			return nil, err
		}
		if err = s.releaseTemporaryCoefficients(remainder); err != nil {
			_ = s.resetPolynomial(&next)
			_ = s.closePolynomialSequence(rows)
			return nil, err
		}
		rows = append(rows, next)
	}
}
