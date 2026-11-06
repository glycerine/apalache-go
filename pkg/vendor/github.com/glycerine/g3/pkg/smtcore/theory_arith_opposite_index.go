package smtcore

import "math/big"

// Index exact opposite inequalities in assertion order. The source detects
// fixed values from variable bounds instead of comparing every unrelated pair.
// Lists are shared by key; iterating them preserves the old first-match order.
func (t *SMTLinearArithmeticTheory) oppositeArithmeticAssertions() [][]int {
	keys := make([]string, len(t.assertions))
	byKey := make(map[string][]int)
	key := func(form *smtLinearForm) string {
		return smtArithmeticCoefficientKey(form) + ";" + form.constant.RatString() + ";" + form.infinity.RatString() + ";" + form.infinitesimal.RatString()
	}
	for i, assertion := range t.assertions {
		form, ok := smtLinearNormalizedLE(&t.atoms[assertion.atom], assertion)
		if !ok {
			continue
		}
		k := key(&form)
		byKey[k] = append(byKey[k], i)
		form.scale(big.NewRat(-1, 1))
		keys[i] = key(&form)
	}
	result := make([][]int, len(keys))
	for i, k := range keys {
		if k != "" {
			result[i] = byKey[k]
		}
	}
	return result
}
