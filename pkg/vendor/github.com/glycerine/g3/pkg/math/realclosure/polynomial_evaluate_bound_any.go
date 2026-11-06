package realclosure

// Source: src/math/realclosure/realclosure.cpp:3788-3865.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// signAnyPolynomialAtBinaryBound is the source's exact Horner fallback. It is
// essential when interval evaluation loses dependency information between
// coefficients from the same algebraic extension.
func (s *realValueStore) signAnyPolynomialAtBinaryBound(coefficients []*realValue, bounds *numeral.BinaryRationalManager, bound *numeral.BinaryRational, usePrem bool) (sign int32, err error) {
	if bounds == nil || bound == nil {
		return 0, arena.ErrInvalidSlot
	}
	if err = s.checkAnyPolynomial(coefficients, true); err != nil || len(coefficients) == 0 {
		return 0, err
	}
	raw := numeral.NewRational()
	if err = bounds.ToRational(s.numbers, bound, &raw); err != nil {
		return 0, errors.Join(err, s.releaseSturmRational(&raw))
	}
	x, err := s.ownNonzeroRational(&raw)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(x)) }()
	acc := coefficients[len(coefficients)-1]
	if err = s.retain(acc); err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(acc)) }()
	for i := len(coefficients) - 2; i >= 0; i-- {
		product, e := s.multiplyAnyCanonicalValues(acc, x, usePrem)
		if e != nil {
			return 0, e
		}
		if e = s.releaseSturmScratch(acc); e != nil {
			_ = s.releaseSturmScratch(product)
			acc = nil
			return 0, e
		}
		acc = nil
		next, e := s.addAnyCanonicalValues(product, coefficients[i], usePrem)
		e = errors.Join(e, s.releaseSturmScratch(product))
		if e != nil {
			_ = s.releaseSturmScratch(next)
			return 0, e
		}
		acc = next
	}
	return s.determineAnyRetainedValueSign(acc, usePrem)
}
