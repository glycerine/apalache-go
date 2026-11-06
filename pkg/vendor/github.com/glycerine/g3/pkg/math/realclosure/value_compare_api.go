package realclosure

// Source: src/math/realclosure/realclosure.cpp:5684-5720 and 6192-6241.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) compareValues(a, b *realValue, bounds *numeral.BinaryRationalManager) (int, error) {
	if a == b {
		if a != nil && (s == nil || a.store != s || a.deleted || a.refs == 0) {
			return 0, arena.ErrInvalidSlot
		}
		return 0, nil
	}
	if a == nil {
		sign, err := s.signValue(b)
		return -int(sign), err
	}
	if b == nil {
		sign, err := s.signValue(a)
		return int(sign), err
	}
	if a.store != s || b.store != s || a.deleted || b.deleted || a.refs == 0 || b.refs == 0 {
		return 0, arena.ErrInvalidSlot
	}
	if a.isRational && b.isRational {
		return s.numbers.Compare(&a.rational, &b.rational)
	}
	if !a.isRational && !b.isRational && a.root == b.root && len(a.functionNumerator.coefficients) == 2 && len(b.functionNumerator.coefficients) == 2 && s.rankedDenominatorIsOne(a.functionDenominator.coefficients) && s.rankedDenominatorIsOne(b.functionDenominator.coefficients) {
		return 0, nil
	}
	if bounds == nil {
		return 0, arena.ErrInvalidSlot
	}
	for precision := s.rationalInitialPrecision(); precision <= s.intervalMaxPrecision(); precision++ {
		ai, err := s.valueInterval(a, bounds)
		if err != nil {
			return 0, err
		}
		bi, err := s.valueInterval(b, bounds)
		if err != nil {
			return 0, err
		}
		before, err := ai.Before(bi)
		if err != nil {
			return 0, err
		}
		if before {
			return -1, nil
		}
		before, err = bi.Before(ai)
		if err != nil {
			return 0, err
		}
		if before {
			return 1, nil
		}
		refinedA, err := s.refineValueInterval(a, bounds, precision+1)
		if err != nil {
			return 0, err
		}
		refinedB, err := s.refineValueInterval(b, bounds, precision+1)
		if err != nil {
			return 0, err
		}
		if !refinedA || !refinedB {
			break
		}
	}
	return 0, arena.ErrInvalidSlot
}

func (s *realValueStore) equalValues(a, b *realValue, bounds *numeral.BinaryRationalManager) (bool, error) {
	comparison, err := s.compareValues(a, b, bounds)
	return comparison == 0, err
}

func (s *realValueStore) lessValues(a, b *realValue, bounds *numeral.BinaryRationalManager) (bool, error) {
	comparison, err := s.compareValues(a, b, bounds)
	return comparison < 0, err
}

func (s *realValueStore) greaterValues(a, b *realValue, bounds *numeral.BinaryRationalManager) (bool, error) {
	comparison, err := s.compareValues(a, b, bounds)
	return comparison > 0, err
}
