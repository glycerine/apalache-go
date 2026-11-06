package polynomial

// Source: src/math/polynomial/polynomial.cpp:5094-5114;
// src/util/mpzzp.h:167-185; src/util/mpz.h:468-485.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerManager_scalarExactAllocationRoots(m *unumeral.IntegerManager, r *polynomialScalarExactDivInvocationReceipt) *polynomialNumericRoots {
	if m == nil || !m.ValidOwner() || r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesScalarExactDivInvocation(r) || !r.frame.executing || r.core.callbackDepth != 0 {
		return nil
	}
	return &r.frame.numeric
}
func numericIntegerManager_scalarExactContextRoots(m *unumeral.IntegerManager, c integerAllocationInvocationContext) *polynomialNumericRoots {
	if c.scalarExact == nil || c.scalarExactInvocation == 0 || c.scalarExactInvocation != c.scalarExact.invocation {
		return nil
	}
	return numericIntegerManager_scalarExactAllocationRoots(m, c.scalarExact)
}
func numericIntegerModularManager_scalarExactAllocationContext(m *unumeral.IntegerModularManager, r *polynomialScalarExactDivInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || m.Integers() == nil || numericIntegerManager_scalarExactAllocationRoots(m.Integers(), r) == nil || r.core.modular != m {
		return integerAllocationInvocationContext{}
	}
	return integerAllocationInvocationContext{scalarExact: r, scalarExactInvocation: r.invocation}
}
func numericIntegerModularManager_divWithScalarExactDivReceipt(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, r *polynomialScalarExactDivInvocationReceipt) error {
	context := numericIntegerModularManager_scalarExactAllocationContext(m, r)
	e := m.WithAllocationContext((context).utility(m.Integers())).Div(a, b, out)
	if m != nil && m.Integers() != nil && numericIntegerManager_scalarExactContextRoots(m.Integers(), context) != nil && r.frame.sourceCause == nil {
		r.frame.sourceCause = numericIntegerManager_scalarExactNumericSourceCause(m.Integers(), r)
	}
	return e
}

// Bind only the child returned by the actual guarded RAW declaration action.
func numericIntegerManager_bindScalarExactDivRaw(m *unumeral.IntegerManager, c integerAllocationInvocationContext, child *unumeral.IntegerRawInvocationReceipt, division bool) {
	if numericIntegerManager_scalarExactContextRoots(m, c) == nil || !m.MatchesRawInvocation(child) {
		return
	}

	if division {
		c.scalarExact.frame.divisionRaw = *child
	} else {
		c.scalarExact.frame.extendedRaw = *child
	}
}

// History is read independently of the allocation/executing predicate.
func numericIntegerManager_scalarExactNumericSourceCause(m *unumeral.IntegerManager, r *polynomialScalarExactDivInvocationReceipt) error {
	if r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesScalarExactDivHistory(r) {
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
func numericIntegerManager_cleanupScalarExactDivNumeric(m *unumeral.IntegerManager, r *polynomialScalarExactDivInvocationReceipt, abort bool) error {
	if r == nil || r.core == nil || r.core.self != r.core || r.core.integer != m || !r.core.matchesScalarExactDivInvocation(r) {
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

// The modular engine erases the integer scoped NUM in the actual nonzero-field
// branch; otherwise the same scoped bool method receives this exact caller.
func numericIntegerModularManager_dividesWithScalarExactDivReceipt(m *unumeral.IntegerModularManager, a, b *unumeral.Int, r *polynomialScalarExactDivInvocationReceipt) (bool, error) {
	result, e := m.WithAllocationContext((numericIntegerModularManager_scalarExactAllocationContext(m, r)).utility(m.Integers())).Divides(a, b)
	if m != nil && m.Integers() != nil && numericIntegerManager_scalarExactAllocationRoots(m.Integers(), r) != nil && r.frame.sourceCause == nil {
		r.frame.sourceCause = m.Integers().DividesNumeralInvocationSourceCause(&r.frame.boolNumeral)
	}
	return result, e
}
func (m *MultivariatePolynomialCore) scalarExactDirectPending(s *polynomialExactDivisionScope) bool {
	return m.integer.MatchesDividesNumeralInvocation(&s.boolNumeral) || m.exactDivDirectPending(s)
}
