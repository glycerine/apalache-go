package realclosure

// Source: src/math/realclosure/realclosure.cpp:3370-3458.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) normalizeRankedIntegerContent(p []*realValue, usePrem bool) (result []*realValue, err error) {
	gcd := numeral.NewInt(0)
	defer func() { err = errors.Join(err, s.releaseSturmInteger(&gcd)) }()
	clean, err := s.polynomialIntegerContent(p, &gcd)
	if err != nil {
		return nil, err
	}
	one, err := s.numbers.Integers().IsOne(&gcd)
	if err != nil {
		return nil, err
	}
	if !clean || one {
		return s.retainRationalPolynomial(p)
	}
	scalar, err := s.ownPositiveIntegerScalar(&gcd)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(scalar)) }()
	return s.divideRankedPolynomialByScalar(p, scalar, usePrem)
}
