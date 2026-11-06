package realclosure

// Source: src/math/realclosure/realclosure.cpp:5727-5761.

import "sort"

func (s *realValueStore) collectRankedAlgebraicRoots(value *realValue) ([]*algebraicRoot, error) {
	seen := make(map[*algebraicRoot]bool)
	var roots []*algebraicRoot
	var markValue func(*realValue) error
	markPolynomial := func(coefficients []*realValue) error {
		for _, coefficient := range coefficients {
			if err := markValue(coefficient); err != nil {
				return err
			}
		}
		return nil
	}
	markValue = func(current *realValue) error {
		if current == nil || current.isRational {
			_, err := s.retainedValueRank(current)
			return err
		}
		if _, err := s.retainedValueRank(current); err != nil {
			return err
		}
		root := current.root
		if !seen[root] {
			seen[root] = true
			roots = append(roots, root)
			if err := markPolynomial(root.polynomial.coefficients); err != nil {
				return err
			}
		}
		if err := markPolynomial(current.functionNumerator.coefficients); err != nil {
			return err
		}
		return markPolynomial(current.functionDenominator.coefficients)
	}
	if err := markValue(value); err != nil {
		return nil, err
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].index < roots[j].index })
	return roots, nil
}
