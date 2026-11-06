package polynomial

// Source: src/math/polynomial/polynomial.cpp:5114-5156;
// src/util/mpzzp.h:167-185; src/util/mpz.h:468-485.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerManager_exactDivAllocationRoots(m *unumeral.IntegerManager, r *polynomialExactDivInvocationReceipt) *polynomialNumericRoots {
	if m == nil || !m.ValidOwner() || r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesExactDivInvocation(r) || !r.frame.executing || r.core.callbackDepth != 0 {
		return nil
	}
	return &r.frame.numeric
}
func numericIntegerManager_exactDivContextRoots(m *unumeral.IntegerManager, c integerAllocationInvocationContext) *polynomialNumericRoots {
	if c.exactDiv == nil || c.exactDivInvocation == 0 || c.exactDivInvocation != c.exactDiv.invocation {
		return nil
	}
	return numericIntegerManager_exactDivAllocationRoots(m, c.exactDiv)
}
func numericIntegerModularManager_exactDivAllocationContext(m *unumeral.IntegerModularManager, r *polynomialExactDivInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || m.Integers() == nil || numericIntegerManager_exactDivAllocationRoots(m.Integers(), r) == nil || r.core.modular != m {
		return integerAllocationInvocationContext{}
	}
	return integerAllocationInvocationContext{exactDiv: r, exactDivInvocation: r.invocation}
}
func numericIntegerModularManager_divWithExactDivReceipt(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, r *polynomialExactDivInvocationReceipt) error {
	context := numericIntegerModularManager_exactDivAllocationContext(m, r)
	e := m.WithAllocationContext((context).utility(m.Integers())).Div(a, b, out)
	if m != nil && m.Integers() != nil && numericIntegerManager_exactDivContextRoots(m.Integers(), context) != nil && r.frame.sourceCause == nil {
		r.frame.sourceCause = numericIntegerManager_exactDivNumericSourceCause(m.Integers(), r)
	}
	return e
}
func numericIntegerModularManager_negWithExactDivReceipt(m *unumeral.IntegerModularManager, out *unumeral.Int, r *polynomialExactDivInvocationReceipt) error {
	if e := m.Check(); e != nil {
		return e
	}
	context := numericIntegerModularManager_exactDivAllocationContext(m, r)
	if e := m.Integers().WithAllocationContext((context).utility(m.Integers())).Neg(out); e != nil {
		return e
	}
	return numericIntegerModularManager_normalizeWithSourceContext(m, out, context)
}

// Bind only the child returned by the actual guarded RAW declaration action.
func numericIntegerManager_bindExactDivRaw(m *unumeral.IntegerManager, c integerAllocationInvocationContext, child *unumeral.IntegerRawInvocationReceipt, division bool) {
	if numericIntegerManager_exactDivContextRoots(m, c) == nil || !m.MatchesRawInvocation(child) {
		return
	}

	if division {
		c.exactDiv.frame.divisionRaw = *child
	} else {
		c.exactDiv.frame.extendedRaw = *child
	}
}

// History is read independently of the allocation/executing predicate.
func numericIntegerManager_exactDivNumericSourceCause(m *unumeral.IntegerManager, r *polynomialExactDivInvocationReceipt) error {
	if r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesExactDivInvocation(r) {
		return nil
	}
	s := r.frame

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
func numericIntegerManager_cleanupExactDivNumeric(m *unumeral.IntegerManager, r *polynomialExactDivInvocationReceipt, abort bool) error {
	if r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesExactDivInvocation(r) {
		return nil
	}
	s := r.frame
	if s.executing {
		return ErrPolynomialPending
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
