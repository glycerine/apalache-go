package qe

import "math/big"

func (c *arithmeticModelProjectCall) findMinMax(bounds []*ArithmeticBound, lower bool) (value *big.Rat, index uint32, found bool, err error) {
	for i, bound := range bounds {
		atom, cause := c.plugin.manager.RefGet(bound.Atom)
		if cause != nil {
			return nil, 0, false, cause
		}
		trueInModel, cause := c.truth(atom)
		if cause != nil {
			return nil, 0, false, cause
		}
		if !trueInModel {
			continue
		}
		term, cause := c.plugin.manager.RefGet(bound.Term)
		if cause != nil {
			return nil, 0, false, cause
		}
		number, cause := c.numeral(term)
		if cause != nil {
			return nil, 0, false, cause
		}
		number.Quo(number, new(big.Rat).Abs(bound.Coefficient))
		if !found || lower && number.Cmp(value) < 0 || !lower && number.Cmp(value) > 0 {
			value, index, found = number, uint32(i), true
		}
	}
	return value, index, found, nil
}
