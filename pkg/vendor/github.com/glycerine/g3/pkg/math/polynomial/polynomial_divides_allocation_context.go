package polynomial

// Source: src/math/polynomial/polynomial.cpp:5154-5194;
// src/util/mpzzp.h:131,192,195-208; src/util/mpz.h:468-485.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerManager_dividesAllocationRoots(m *unumeral.IntegerManager, r *polynomialDividesInvocationReceipt) *polynomialNumericRoots {
	if m == nil || !m.ValidOwner() || r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesDividesInvocation(r) || r.frame.constructed < 2 || !r.frame.executing || r.core.callbackDepth != 0 {
		return nil
	}
	return &r.frame.numeric
}
func numericIntegerManager_dividesContextRoots(m *unumeral.IntegerManager, c integerAllocationInvocationContext) *polynomialNumericRoots {
	if c.divides == nil || c.dividesInvocation == 0 || c.dividesInvocation != c.divides.invocation {
		return nil
	}
	return numericIntegerManager_dividesAllocationRoots(m, c.divides)
}
func numericIntegerModularManager_dividesAllocationContext(m *unumeral.IntegerModularManager, r *polynomialDividesInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || m.Integers() == nil || numericIntegerManager_dividesAllocationRoots(m.Integers(), r) == nil || r.core.modular != m {
		return integerAllocationInvocationContext{}
	}
	return integerAllocationInvocationContext{divides: r, dividesInvocation: r.invocation}
}
func numericIntegerModularManager_divWithDividesReceipt(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, r *polynomialDividesInvocationReceipt) error {
	context := numericIntegerModularManager_dividesAllocationContext(m, r)
	e := m.WithAllocationContext((context).utility(m.Integers())).Div(a, b, out)
	if m != nil && m.Integers() != nil && numericIntegerManager_dividesContextRoots(m.Integers(), context) != nil && r.frame.sourceCause == nil {
		r.frame.sourceCause = numericIntegerManager_dividesNumericSourceCause(m.Integers(), r)
	}
	return e
}
func numericIntegerModularManager_negWithDividesReceipt(m *unumeral.IntegerModularManager, out *unumeral.Int, r *polynomialDividesInvocationReceipt) error {
	if e := m.Check(); e != nil {
		return e
	}
	context := numericIntegerModularManager_dividesAllocationContext(m, r)
	if e := m.Integers().WithAllocationContext((context).utility(m.Integers())).Neg(out); e != nil {
		return e
	}
	return numericIntegerModularManager_normalizeWithSourceContext(m, out, context)
}

// Bind only the child returned by the actual guarded RAW declaration action.
func numericIntegerManager_bindDividesRaw(m *unumeral.IntegerManager, c integerAllocationInvocationContext, child *unumeral.IntegerRawInvocationReceipt, division bool) {
	if numericIntegerManager_dividesContextRoots(m, c) == nil || !m.MatchesRawInvocation(child) {
		return
	}

	if division {
		c.divides.frame.divisionRaw = *child
	} else {
		c.divides.frame.extendedRaw = *child
	}
}

// History is read independently of the allocation/executing predicate.
func numericIntegerManager_dividesNumericSourceCause(m *unumeral.IntegerManager, r *polynomialDividesInvocationReceipt) error {
	if r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesDividesInvocationHistory(r) {
		return nil
	}
	s := r.frame
	if e := m.DividesNumeralInvocationSourceCause(&s.boolNumeral); e != nil {
		return e
	}

	if e := m.RawInvocationSourceCause(&s.extendedRaw); e != nil {
		return e
	}
	if e := m.RawInvocationSourceCause(&s.divisionRaw); e != nil {
		return e
	}
	return nil
}

// False completes reached younger source children. True drains unjoined
// acquisitions only when the caller has completed its older lexical suffix.
func numericIntegerManager_cleanupDividesNumeric(m *unumeral.IntegerManager, r *polynomialDividesInvocationReceipt, abort bool) error {
	if r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesDividesInvocationHistory(r) {
		return nil
	}
	s := r.frame
	if s.executing {
		return ErrPolynomialPending
	}
	if e := m.CleanupDividesNumeralInvocation(&s.boolNumeral, abort); e != nil {
		return e
	}
	if e := r.core.modular.CleanupPNormalizeInvocation(&s.normalize, abort); e != nil {
		return e
	}
	if e := m.CleanupRawInvocation(&s.extendedRaw, abort); e != nil {
		return e
	}
	if e := m.CleanupRawInvocation(&s.divisionRaw, abort); e != nil {
		return e
	}

	if abort {

		return s.numeric.cleanup(m)
	}
	return nil
}

func numericIntegerModularManager_dividesWithDividesReceipt(m *unumeral.IntegerModularManager, a, b *unumeral.Int, r *polynomialDividesInvocationReceipt) (bool, error) {
	context := numericIntegerModularManager_dividesAllocationContext(m, r)
	result, e := m.WithAllocationContext((context).utility(m.Integers())).Divides(a, b)
	if m != nil && m.Integers() != nil && numericIntegerManager_dividesContextRoots(m.Integers(), context) != nil && r.frame.sourceCause == nil {
		r.frame.sourceCause = numericIntegerManager_dividesNumericSourceCause(m.Integers(), r)
	}
	return result, e
}
func numericIntegerManager_dividesNumericPending(m *unumeral.IntegerManager, r *polynomialDividesInvocationReceipt) bool {
	if r == nil || r.core == nil || r.core.integer != m || !r.core.matchesDividesInvocationHistory(r) {
		return false
	}
	s := r.frame
	if polynomialNegNumericPending(&s.numeric) {
		return true
	}
	if m.MatchesDividesNumeralInvocation(&s.boolNumeral) {
		if s.boolNumeral.Pending() {
			return true
		}
	}
	if m.MatchesRawInvocation(&s.extendedRaw) || m.MatchesRawInvocation(&s.divisionRaw) {
		return true
	}
	if r.core.modular.MatchesPNormalizeInvocation(&s.normalize) {
		return true
	}

	return false
}
