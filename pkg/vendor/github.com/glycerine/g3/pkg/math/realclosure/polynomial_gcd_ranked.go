package realclosure

// Source: src/math/realclosure/realclosure.cpp:3460-3525.

import "errors"

func (s *realValueStore) gcdRankedPolynomials(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	a, err := s.retainRationalPolynomial(left)
	if err != nil {
		return nil, err
	}
	b, err := s.retainRationalPolynomial(right)
	if err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(a))
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(a), s.releaseSturmCoefficients(b)) }()
	for len(b) > 0 {
		quotient, remainder, e := s.divideRankedPolynomials(a, b, usePrem)
		e = errors.Join(e, s.releaseSturmCoefficients(quotient))
		if e != nil {
			_ = s.releaseSturmCoefficients(remainder)
			return nil, e
		}
		if e = s.releaseSturmCoefficients(a); e != nil {
			_ = s.releaseSturmCoefficients(remainder)
			a = nil
			return nil, e
		}
		a, b = b, remainder
	}
	return s.makeRankedPolynomialMonic(a, usePrem)
}
