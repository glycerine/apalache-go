package realclosure

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) normalizedPseudoQuerySequence(p, q []*realValue) ([]realPolynomial, error) {
	if s == nil || len(p) == 0 || p[len(p)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	derivative, err := s.derivativePolynomial(p)
	if err != nil {
		return nil, err
	}
	if len(derivative) == 0 {
		return nil, s.releaseTemporaryCoefficients(derivative)
	}
	second := derivative
	if q != nil {
		second, err = s.multiplyNormalizedPolynomials(derivative, q)
		releaseErr := s.releaseTemporaryCoefficients(derivative)
		if err != nil {
			return nil, err
		}
		if releaseErr != nil {
			_ = s.releaseTemporaryCoefficients(second)
			return nil, releaseErr
		}
	}
	rows, err := s.sturmSequencePseudoNormalized(p, second)
	releaseErr := s.releaseTemporaryCoefficients(second)
	if err != nil {
		return nil, err
	}
	if releaseErr != nil {
		_ = s.closePolynomialSequence(rows)
		return nil, releaseErr
	}
	return rows, nil
}
