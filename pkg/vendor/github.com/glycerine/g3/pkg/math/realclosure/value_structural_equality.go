package realclosure

// Source: src/math/realclosure/realclosure.cpp:3034-3087.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) structurallyEqualValues(a, b *realValue) (bool, error) {
	if a == b {
		if a != nil && (s == nil || a.store != s || a.deleted || a.refs == 0) {
			return false, arena.ErrInvalidSlot
		}
		return true, nil
	}
	if a == nil || b == nil {
		return false, nil
	}
	if s == nil || a.store != s || b.store != s || a.deleted || b.deleted || a.refs == 0 || b.refs == 0 {
		return false, arena.ErrInvalidSlot
	}
	if a.isRational || b.isRational {
		if !a.isRational || !b.isRational {
			return false, nil
		}
		return s.numbers.Equal(&a.rational, &b.rational)
	}
	if a.root != b.root {
		return false, nil
	}
	numeratorsEqual, err := s.structurallyEqualPolynomials(a.functionNumerator.coefficients, b.functionNumerator.coefficients)
	if err != nil || !numeratorsEqual {
		return numeratorsEqual, err
	}
	return s.structurallyEqualPolynomials(a.functionDenominator.coefficients, b.functionDenominator.coefficients)
}

func (s *realValueStore) structurallyEqualPolynomials(a, b []*realValue) (bool, error) {
	if len(a) != len(b) {
		return false, nil
	}
	for i := range a {
		equal, err := s.structurallyEqualValues(a[i], b[i])
		if err != nil || !equal {
			return equal, err
		}
	}
	return true, nil
}
