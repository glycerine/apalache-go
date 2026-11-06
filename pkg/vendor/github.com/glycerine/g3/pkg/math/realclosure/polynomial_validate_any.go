package realclosure

// Source: src/math/realclosure/realclosure.cpp:752-776 polynomial invariants.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) checkAnyPolynomial(p []*realValue, allowEmpty bool) error {
	if s == nil || (!allowEmpty && len(p) == 0) || len(p) > 0 && p[len(p)-1] == nil {
		return arena.ErrInvalidSlot
	}
	for _, coefficient := range p {
		if coefficient != nil && (coefficient.store != s || coefficient.deleted || coefficient.refs == 0) {
			return arena.ErrInvalidSlot
		}
	}
	return nil
}

func trimAnyPolynomial(p []*realValue) []*realValue {
	for len(p) > 0 && p[len(p)-1] == nil {
		p = p[:len(p)-1]
	}
	return p
}
