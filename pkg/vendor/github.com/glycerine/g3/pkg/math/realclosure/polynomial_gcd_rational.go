package realclosure

// Source: src/math/realclosure/realclosure.cpp:3460-3525.

import "errors"

func (s *realValueStore) makeRationalPolynomialMonic(p []*realValue) (result []*realValue, err error) {
	if err = s.checkRationalPolynomial(p, true); err != nil {
		return nil, err
	}
	if len(p) == 0 {
		return nil, nil
	}
	leading := p[len(p)-1]
	result = make([]*realValue, len(p))
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i := range p {
		if i+1 == len(p) {
			result[i], err = s.newRational(1, 1)
		} else {
			result[i], err = s.divideRationals(p[i], leading)
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *realValueStore) gcdRationalPolynomials(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	if err = s.checkRationalPolynomial(left, true); err != nil {
		return nil, err
	}
	if err = s.checkRationalPolynomial(right, true); err != nil {
		return nil, err
	}
	retainCopy := func(p []*realValue) ([]*realValue, error) {
		copyP := append([]*realValue(nil), p...)
		for i, coefficient := range copyP {
			if e := s.retain(coefficient); e != nil {
				_ = s.releaseSturmCoefficients(copyP[:i])
				return nil, e
			}
		}
		return copyP, nil
	}
	a, err := retainCopy(left)
	if err != nil {
		return nil, err
	}
	b, err := retainCopy(right)
	if err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(a))
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(a), s.releaseSturmCoefficients(b)) }()
	for len(b) > 0 {
		var next []*realValue
		if usePrem {
			next, err = s.signedPseudoRemainderRational(a, b)
		} else {
			next, err = s.signedRemainderRational(a, b)
		}
		if err != nil {
			return nil, err
		}
		if err = s.releaseSturmCoefficients(a); err != nil {
			_ = s.releaseSturmCoefficients(next)
			a = nil
			return nil, err
		}
		a = b
		b = next
	}
	result, err = s.makeRationalPolynomialMonic(a)
	return result, err
}
