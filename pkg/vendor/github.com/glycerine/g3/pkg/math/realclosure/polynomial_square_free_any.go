package realclosure

// Source: src/math/realclosure/realclosure.cpp:3610-3629.

import "errors"

func (s *realValueStore) squareFreeAnyPolynomial(p []*realValue, usePrem bool) (result []*realValue, err error) {
	if err = s.checkAnyPolynomial(p, false); err != nil {
		return nil, err
	}
	restoreAuxiliary := s.withAuxiliaryValues()
	defer restoreAuxiliary()
	if len(p) <= 1 {
		return s.retainRationalPolynomial(p)
	}
	derivative, err := s.derivativeAnyPolynomial(p, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(derivative)) }()
	gcd, err := s.gcdAnyPolynomials(p, derivative, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(gcd)) }()
	if len(gcd) <= 1 {
		return s.retainRationalPolynomial(p)
	}
	result, remainder, err := s.divideAnyPolynomials(p, gcd, usePrem)
	if err != nil {
		return nil, err
	}
	if len(remainder) != 0 {
		return nil, errors.Join(errors.New("realclosure: non-exact algebraic square-free division"), s.releaseSturmCoefficients(result), s.releaseSturmCoefficients(remainder))
	}
	return result, nil
}
