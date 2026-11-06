package realclosure

// Source: src/math/realclosure/realclosure.cpp:3590-3606.

import "errors"

func (s *realValueStore) derivativeAnyPolynomial(p []*realValue, usePrem bool) (result []*realValue, err error) {
	if len(p) <= 1 {
		return nil, nil
	}
	result = make([]*realValue, len(p)-1)
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i := 1; i < len(p); i++ {
		factor, e := s.newRational(int64(i), 1)
		if e != nil {
			return nil, e
		}
		result[i-1], e = s.multiplyAnyCanonicalValues(factor, p[i], usePrem)
		e = errors.Join(e, s.releaseSturmScratch(factor))
		if e != nil {
			return nil, e
		}
	}
	return trimAnyPolynomial(result), nil
}
