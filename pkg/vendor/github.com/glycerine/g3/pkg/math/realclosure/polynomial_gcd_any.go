package realclosure

// Source: src/math/realclosure/realclosure.cpp:3497-3525.

import "errors"

func (s *realValueStore) gcdAnyPolynomials(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	a, e := s.retainRationalPolynomial(left)
	if e != nil {
		return nil, e
	}
	b, e := s.retainRationalPolynomial(right)
	if e != nil {
		return nil, errors.Join(e, s.releaseSturmCoefficients(a))
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(a), s.releaseSturmCoefficients(b)) }()
	for len(b) > 0 {
		q, r, x := s.divideAnyPolynomials(a, b, usePrem)
		x = errors.Join(x, s.releaseSturmCoefficients(q))
		if x != nil {
			_ = s.releaseSturmCoefficients(r)
			return nil, x
		}
		if x = s.releaseSturmCoefficients(a); x != nil {
			_ = s.releaseSturmCoefficients(r)
			a = nil
			return nil, x
		}
		a, b = b, r
	}
	return s.makeAnyPolynomialMonic(a, usePrem)
}
