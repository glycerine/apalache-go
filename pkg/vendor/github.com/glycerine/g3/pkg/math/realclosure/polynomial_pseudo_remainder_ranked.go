package realclosure

// Source: src/math/realclosure/realclosure.cpp:2917-2988, 3020-3043.

import (
	"errors"
	"fmt"
)

func (s *realValueStore) signedPseudoRemainderRanked(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
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
		steps++
		shift := len(result) - len(right)
		high := result[len(result)-1]
		if err = s.retain(high); err != nil {
			return fail(err)
		}
		for i := 0; i+1 < len(result); i++ {
			updated, e := s.multiplyRankedRetainedValues(result[i], leading, usePrem)
			if e != nil {
				_ = s.releaseSturmScratch(high)
				return fail(fmt.Errorf("ranked prem scale coefficient: %w", e))
			}
			if e = s.releaseSturmScratch(result[i]); e != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(high)
				return fail(e)
			}
			result[i] = updated
		}
		for i := 0; i+1 < len(right); i++ {
			product, e := s.multiplyRankedRetainedValues(high, right[i], usePrem)
			if e != nil {
				_ = s.releaseSturmScratch(high)
				return fail(fmt.Errorf("ranked prem product: %w", e))
			}
			index := shift + i
			// The value is independent of the remainder strategy. Use the exact
			// field-remainder sign route here to avoid recursively entering this
			// pseudo-remainder while canonicalizing a sibling-extension zero.
			updated, e := s.subtractTowerCanonicalValues(result[index], product, false)
			if e != nil {
				_ = s.releaseSturmScratch(high)
				if product != nil && !product.deleted {
					_ = s.releaseSturmScratch(product)
				}
				return fail(fmt.Errorf("ranked prem subtraction body: %w", e))
			}
			// Tower canonicalization may collapse an aliased temporary all the
			// way to zero. In that case its last reference is already consumed.
			if product != nil && !product.deleted {
				e = s.releaseSturmScratch(product)
			}
			if e != nil {
				_ = s.releaseSturmScratch(high)
				return fail(fmt.Errorf("ranked prem product cleanup: %w", e))
			}
			if result[index] != nil && result[index].deleted {
				result[index] = nil
			} else if e = s.releaseSturmScratch(result[index]); e != nil {
				_ = s.releaseSturmScratch(updated)
				_ = s.releaseSturmScratch(high)
				return fail(fmt.Errorf("ranked prem old coefficient cleanup: %w", e))
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
	leadingSign, err := s.determineTowerRetainedValueSign(leading, usePrem)
	if err != nil {
		return fail(fmt.Errorf("ranked prem leading sign: %w", err))
	}
	if steps%2 == 0 || leadingSign > 0 {
		negated := make([]*realValue, len(result))
		for i, coefficient := range result {
			negated[i], err = s.negateRankedRetainedValue(coefficient, usePrem)
			if err != nil {
				_ = s.releaseSturmCoefficients(negated)
				return fail(err)
			}
		}
		if err = s.releaseSturmCoefficients(result); err != nil {
			_ = s.releaseSturmCoefficients(negated)
			return nil, err
		}
		result = negated
	}
	normalized, err := s.normalizeRankedIntegerContent(result, usePrem)
	if err != nil {
		return fail(fmt.Errorf("ranked prem normalization: %w", err))
	}
	if err = s.releaseSturmCoefficients(result); err != nil {
		_ = s.releaseSturmCoefficients(normalized)
		return nil, err
	}
	return normalized, nil
}
