package realclosure

// Source: src/math/realclosure/realclosure.cpp:3512-3574.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) towerPolynomialIntegerContent(p []*realValue, gcd *numeral.Int) (bool, error) {
	if len(p) == 0 || gcd == nil {
		return false, arena.ErrInvalidSlot
	}
	found := false
	for _, coefficient := range p {
		if coefficient == nil {
			continue
		}
		clean, err := s.accumulateIntegerContent(coefficient, gcd)
		if err != nil || !clean {
			return false, err
		}
		found = true
		one, err := s.numbers.Integers().IsOne(gcd)
		if err != nil || one {
			return found, err
		}
	}
	return found, nil
}
