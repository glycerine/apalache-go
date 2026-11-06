package realclosure

// Source: src/math/realclosure/realclosure.cpp:3090-3165, concrete retained
// rational value specialization.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) hasCleanDenominator(value *realValue) (bool, error) {
	if value == nil {
		return true, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 {
		return false, arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.numbers.IsInt(&value.rational)
	}
	// Live algebraic values have denominator one. Recurse through their
	// numerator, as the source does for rational_function_value.
	for _, coefficient := range value.functionNumerator.coefficients {
		clean, err := s.hasCleanDenominator(coefficient)
		if err != nil || !clean {
			return clean, err
		}
	}
	return true, nil
}

func (s *realValueStore) cleanRationalValueDenominator(value *realValue) (numerator, denominator *realValue, err error) {
	if !s.rationalOperand(value) {
		return nil, nil, arena.ErrInvalidSlot
	}
	denominator, err = s.newRational(1, 1)
	if err != nil {
		return nil, nil, err
	}
	if value == nil {
		return nil, denominator, nil
	}
	n, d := numeral.NewRational(), numeral.NewRational()
	defer func() {
		if n.Num.Val != 0 {
			err = errors.Join(err, s.releaseSturmRational(&n))
		}
		if d.Num.Val != 0 {
			err = errors.Join(err, s.releaseSturmRational(&d))
		}
		if err != nil {
			err = errors.Join(err, s.releaseSturmScratch(numerator), s.releaseSturmScratch(denominator))
			numerator, denominator = nil, nil
		}
	}()
	if err = s.numbers.NumeratorRational(&value.rational, &n); err != nil {
		return nil, nil, err
	}
	if err = s.numbers.DenominatorRational(&value.rational, &d); err != nil {
		return nil, nil, err
	}
	if err = s.releaseSturmScratch(denominator); err != nil {
		denominator = nil
		return nil, nil, err
	}
	denominator = nil
	numerator, err = s.ownNonzeroRational(&n)
	if err != nil {
		return nil, nil, err
	}
	n = numeral.NewRational()
	denominator, err = s.ownNonzeroRational(&d)
	if err != nil {
		return nil, nil, err
	}
	d = numeral.NewRational()
	return numerator, denominator, nil
}
