package realclosure

// Source: src/math/realclosure/realclosure.cpp:2917-2988, 3020-3043.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// signedPseudoRemainderAny avoids introducing inverses while eliminating the
// high terms. The source sign correction depends on the number of eliminations
// and the sign of the divisor's leading coefficient.
func (s *realValueStore) signedPseudoRemainderAny(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	if err = s.checkAnyPolynomial(left, true); err != nil {
		return nil, err
	}
	if err = s.checkAnyPolynomial(right, false); err != nil {
		return nil, err
	}
	if len(right) == 1 {
		return nil, nil
	}
	result, err = s.retainRationalPolynomial(left)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) ([]*realValue, error) {
		return nil, errors.Join(cause, s.releaseSturmCoefficients(result))
	}
	leading := right[len(right)-1]
	steps := 0
	for len(result) >= len(right) {
		if err = s.checkpoint(); err != nil {
			return fail(err)
		}
		steps++
		shift := len(result) - len(right)
		high := result[len(result)-1]
		if err = s.retain(high); err != nil {
			return fail(err)
		}
		for i := 0; i+1 < len(result); i++ {
			updated, e := s.multiplyAnyCanonicalValues(result[i], leading, usePrem)
			if e != nil {
				_ = s.releaseSturmScratch(high)
				return fail(e)
			}
			if e = s.releaseSturmScratch(result[i]); e != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(high)
				return fail(e)
			}
			result[i] = updated
		}
		for i := 0; i+1 < len(right); i++ {
			product, e := s.multiplyAnyCanonicalValues(high, right[i], usePrem)
			if e != nil {
				_ = s.releaseSturmScratch(high)
				return fail(e)
			}
			index := shift + i
			updated, e := s.subtractAnyCanonicalValues(result[index], product, usePrem)
			e = errors.Join(e, s.releaseSturmScratch(product))
			if e != nil {
				_ = s.releaseSturmScratch(high)
				return fail(e)
			}
			if e = s.releaseSturmScratch(result[index]); e != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(high)
				return fail(e)
			}
			result[index] = updated
		}
		if err = s.releaseSturmScratch(high); err != nil {
			return fail(err)
		}
		if err = s.releaseSturmScratch(result[len(result)-1]); err != nil {
			return fail(err)
		}
		result = trimAnyPolynomial(result[:len(result)-1])
	}
	leadingSign, err := s.determineAnyRetainedValueSign(leading, usePrem)
	if err != nil {
		return fail(err)
	}
	if leadingSign == 0 {
		return fail(arena.ErrInvalidSlot)
	}
	if steps%2 == 0 || leadingSign > 0 {
		negated, e := s.negateAnyPolynomial(result)
		if e != nil {
			return fail(e)
		}
		if e = s.releaseSturmCoefficients(result); e != nil {
			_ = s.releaseSturmCoefficients(negated)
			return nil, e
		}
		result = negated
	}
	return result, nil
}
