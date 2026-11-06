package realclosure

// Source: src/math/realclosure/realclosure.cpp:3167-3288, concrete rational
// coefficient specialization. The source first takes the LCM of all integer
// denominators, then applies the corresponding multiplier to each numerator.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) hasCleanPolynomialDenominators(p []*realValue) (bool, error) {
	if len(p) == 0 {
		return false, arena.ErrInvalidSlot
	}
	for _, coefficient := range p {
		clean, err := s.hasCleanDenominator(coefficient)
		if err != nil || !clean {
			return clean, err
		}
	}
	return true, nil
}

func (s *realValueStore) cleanRationalPolynomialDenominators(p []*realValue) (normalized []*realValue, denominator *realValue, err error) {
	if err = s.checkRationalPolynomial(p, false); err != nil {
		return nil, nil, err
	}
	lcm := numeral.NewRational()
	defer func() {
		if lcm.Num.Val != 0 {
			err = errors.Join(err, s.releaseSturmRational(&lcm))
		}
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(normalized), s.releaseSturmScratch(denominator))
			normalized, denominator = nil, nil
		}
	}()
	if err = s.numbers.SetInt32(&lcm, 1); err != nil {
		return nil, nil, err
	}
	for _, coefficient := range p {
		if coefficient == nil {
			continue
		}
		d := numeral.NewRational()
		if err = s.numbers.DenominatorRational(&coefficient.rational, &d); err != nil {
			return nil, nil, errors.Join(err, s.releaseSturmRational(&d))
		}
		next := numeral.NewRational()
		if err = s.numbers.LCM(&lcm, &d, &next); err != nil {
			return nil, nil, errors.Join(err, s.releaseSturmRational(&d), s.releaseSturmRational(&next))
		}
		if err = s.releaseSturmRational(&d); err != nil {
			return nil, nil, errors.Join(err, s.releaseSturmRational(&next))
		}
		if err = s.releaseSturmRational(&lcm); err != nil {
			lcm = numeral.NewRational()
			return nil, nil, errors.Join(err, s.releaseSturmRational(&next))
		}
		lcm = next
	}
	denominator, err = s.ownNonzeroRational(&lcm)
	if err != nil {
		return nil, nil, err
	}
	lcm = numeral.NewRational()
	normalized = make([]*realValue, len(p))
	for i, coefficient := range p {
		normalized[i], err = s.multiplyRationals(coefficient, denominator)
		if err != nil {
			return nil, nil, err
		}
	}
	return normalized, denominator, nil
}
