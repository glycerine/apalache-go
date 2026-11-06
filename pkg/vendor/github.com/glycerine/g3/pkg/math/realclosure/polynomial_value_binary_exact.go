package realclosure

// Source: src/math/realclosure/realclosure.cpp:3949-3973.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// exactPolynomialValueAtBinaryBound evaluates a retained tower polynomial by
// the source Horner order. The returned value owns one reference.
func (s *realValueStore) exactPolynomialValueAtBinaryBound(coefficients []*realValue, bounds *numeral.BinaryRationalManager, bound *numeral.BinaryRational, usePrem bool) (result *realValue, err error) {
	if len(coefficients) == 0 || coefficients[len(coefficients)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	argument, err := s.ownBinaryBoundValue(bounds, bound)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(argument)) }()
	if err = s.retain(coefficients[len(coefficients)-1]); err != nil {
		return nil, err
	}
	result = coefficients[len(coefficients)-1]
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmScratch(result))
			result = nil
		}
	}()
	for i := len(coefficients) - 2; i >= 0; i-- {
		product, e := s.multiplyRankedRetainedValues(result, argument, usePrem)
		e = errors.Join(e, s.releaseSturmScratch(result))
		result = nil
		if e != nil {
			return nil, e
		}
		result, e = s.addRankedRetainedValues(product, coefficients[i], usePrem)
		e = errors.Join(e, s.releaseSturmScratch(product))
		if e != nil {
			return nil, e
		}
	}
	return result, nil
}
