package realclosure

// Source: src/math/realclosure/realclosure.cpp:3590-3629.

import "errors"

func (s *realValueStore) squareFreeRationalPolynomial(p []*realValue, usePrem bool) (result []*realValue, err error) {
	if err = s.checkRationalPolynomial(p, false); err != nil {
		return nil, err
	}
	restoreAuxiliary := s.withAuxiliaryValues()
	defer restoreAuxiliary()
	if len(p) <= 1 {
		if err = s.retain(p[0]); err != nil {
			return nil, err
		}
		return append([]*realValue(nil), p...), nil
	}
	derivative, err := s.derivativeRationalPolynomial(p)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(derivative)) }()
	gcd, err := s.gcdRationalPolynomials(p, derivative, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(gcd)) }()
	if len(gcd) <= 1 {
		result = append([]*realValue(nil), p...)
		for i, coefficient := range result {
			if err = s.retain(coefficient); err != nil {
				_ = s.releaseSturmCoefficients(result[:i])
				return nil, err
			}
		}
		return result, nil
	}
	result, remainder, err := s.divideRationalPolynomial(p, gcd)
	if err != nil {
		return nil, err
	}
	if len(remainder) != 0 {
		return nil, errors.Join(errors.New("realclosure: non-exact square-free division"), s.releaseSturmCoefficients(result), s.releaseSturmCoefficients(remainder))
	}
	return result, nil
}
