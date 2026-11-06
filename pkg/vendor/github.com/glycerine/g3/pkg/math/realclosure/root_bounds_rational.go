package realclosure

// Source: src/math/realclosure/realclosure.cpp:1383-1580.
//
// These are the concrete rational-coefficient instantiations used by the
// retained root-isolation path.  The source computes Knuth bounds from the
// current coefficient intervals, rather than from exact quotient roots.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func absLowerMagnitude(v *BinaryInterval) (int32, bool, error) {
	if err := v.open(); err != nil {
		return 0, false, err
	}
	positive, err := v.IsP()
	if err != nil {
		return 0, false, err
	}
	if positive {
		zero, err := v.numbers.IsZero(&v.lower)
		if err != nil || zero {
			return 0, false, err
		}
		m, err := v.numbers.MagnitudeLower(&v.lower)
		return m, err == nil, err
	}
	negative, err := v.IsN()
	if err != nil || !negative {
		return 0, false, err
	}
	zero, err := v.numbers.IsZero(&v.upper)
	if err != nil || zero {
		return 0, false, err
	}
	tmp := numeral.NewBinaryRational()
	if err = v.numbers.Copy(&tmp, &v.upper); err == nil {
		err = v.numbers.Neg(&tmp)
	}
	if err != nil {
		return 0, false, errors.Join(err, v.numbers.Del(&tmp))
	}
	m, magErr := v.numbers.MagnitudeLower(&tmp)
	return m, magErr == nil, errors.Join(magErr, v.numbers.Del(&tmp))
}

func absUpperMagnitude(v *BinaryInterval) (int32, bool, error) {
	if err := v.open(); err != nil {
		return 0, false, err
	}
	positive, err := v.IsP()
	if err != nil {
		return 0, false, err
	}
	if positive {
		if v.upperInf {
			return 0, false, nil
		}
		m, err := v.numbers.MagnitudeUpper(&v.upper)
		return m, err == nil, err
	}
	negative, err := v.IsN()
	if err != nil || !negative {
		return 0, false, err
	}
	if v.lowerInf {
		return 0, false, nil
	}
	tmp := numeral.NewBinaryRational()
	if err = v.numbers.Copy(&tmp, &v.lower); err == nil {
		err = v.numbers.Neg(&tmp)
	}
	if err != nil {
		return 0, false, errors.Join(err, v.numbers.Del(&tmp))
	}
	m, magErr := v.numbers.MagnitudeUpper(&tmp)
	return m, magErr == nil, errors.Join(magErr, v.numbers.Del(&tmp))
}

func (s *realValueStore) coefficientMagnitude(v *realValue, bounds *numeral.BinaryRationalManager, lower bool) (int32, bool, error) {
	i, err := s.rationalApproxInterval(v, bounds)
	if err != nil {
		return 0, false, err
	}
	var m int32
	var ok bool
	if lower {
		m, ok, err = absLowerMagnitude(i)
	} else {
		m, ok, err = absUpperMagnitude(i)
	}
	return m, ok, errors.Join(err, i.Close())
}

func (s *realValueStore) positiveRootUpperBound(p []*realValue, bounds *numeral.BinaryRationalManager) (int32, bool, error) {
	if len(p) < 2 || p[len(p)-1] == nil || bounds == nil {
		return 0, false, arena.ErrInvalidSlot
	}
	lcSign, err := s.signValue(p[len(p)-1])
	if err != nil {
		return 0, false, err
	}
	lcMag, ok, err := s.coefficientMagnitude(p[len(p)-1], bounds, true)
	if err != nil || !ok {
		return 0, ok, err
	}
	result := -int32(s.rationalInitialPrecision())
	for k := 2; k <= len(p); k++ {
		a := p[len(p)-k]
		if a == nil {
			continue
		}
		sign, err := s.signValue(a)
		if err != nil {
			return 0, false, err
		}
		if sign == lcSign {
			continue
		}
		aMag, ok, err := s.coefficientMagnitude(a, bounds, false)
		if err != nil || !ok {
			return 0, ok, err
		}
		candidate := (aMag-lcMag)/int32(k) + 2
		if candidate > result {
			result = candidate
		}
	}
	return result, true, nil
}

func (s *realValueStore) negativeRootLowerBound(p []*realValue, bounds *numeral.BinaryRationalManager) (int32, bool, error) {
	if len(p) < 2 || p[len(p)-1] == nil || bounds == nil {
		return 0, false, arena.ErrInvalidSlot
	}
	lcSign, err := s.signValue(p[len(p)-1])
	if err != nil {
		return 0, false, err
	}
	if (len(p)-1)&1 == 0 {
		lcSign = -lcSign
	}
	lcMag, ok, err := s.coefficientMagnitude(p[len(p)-1], bounds, true)
	if err != nil || !ok {
		return 0, ok, err
	}
	result := -int32(s.rationalInitialPrecision())
	for k := 2; k <= len(p); k++ {
		index := len(p) - k
		a := p[index]
		if a == nil {
			continue
		}
		sign, err := s.signValue(a)
		if err != nil {
			return 0, false, err
		}
		if index&1 == 0 {
			sign = -sign
		}
		if sign == lcSign {
			continue
		}
		aMag, ok, err := s.coefficientMagnitude(a, bounds, false)
		if err != nil || !ok {
			return 0, ok, err
		}
		candidate := (aMag-lcMag)/int32(k) + 2
		if candidate > result {
			result = candidate
		}
	}
	return result, true, nil
}

func reverseRealPolynomial(p []*realValue) []*realValue {
	r := make([]*realValue, len(p))
	for i := range p {
		r[len(p)-1-i] = p[i]
	}
	return r
}

func (s *realValueStore) positiveRootLowerBound(p []*realValue, bounds *numeral.BinaryRationalManager) (int32, bool, error) {
	m, ok, err := s.positiveRootUpperBound(reverseRealPolynomial(p), bounds)
	return -m, ok, err
}

func (s *realValueStore) negativeRootUpperBound(p []*realValue, bounds *numeral.BinaryRationalManager) (int32, bool, error) {
	m, ok, err := s.negativeRootLowerBound(reverseRealPolynomial(p), bounds)
	return -m, ok, err
}
