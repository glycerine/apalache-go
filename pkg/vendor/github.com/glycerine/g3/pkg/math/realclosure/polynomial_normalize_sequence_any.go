package realclosure

// Source: src/math/realclosure/realclosure.cpp:3401-3420.

import "errors"

func (s *realValueStore) normalizeSequencePolynomialAny(p []*realValue, usePrem bool) ([]*realValue, error) {
	normalized, _, err := s.normalizeAnyIntegerContent(p, usePrem)
	if err != nil {
		return nil, err
	}
	if err := s.releaseSturmCoefficients(p); err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(normalized))
	}
	return normalized, nil
}
