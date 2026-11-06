package polynomial

// Source: src/math/polynomial/polynomial.cpp:5904-5940;
// src/util/mpzzp.h:167-185,211; src/util/mpz.cpp:730-817.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) hasNormalizeInvocationHistory(r *polynomialNormalizeInvocationReceipt) bool {
	return m != nil && m.self == m && m.core != nil && m.core.self == m.core && r != nil && r.manager == m && r.frame == &m.normalization && r.invocation != 0 && r.invocation == r.frame.invocation
}
func numericIntegerManager_normalizationAllocationRoots(m *unumeral.IntegerManager, r *polynomialNormalizeInvocationReceipt) *polynomialNumericRoots {
	if m == nil || !m.ValidOwner() || r == nil || r.manager == nil || !r.manager.hasNormalizeInvocationHistory(r) || r.manager.core.integer != m || !r.frame.active || !r.frame.executing || r.manager.core.callbackDepth != 0 {
		return nil
	}
	return &r.frame.numeric
}
func (c integerAllocationInvocationContext) normalizationReceipt(m *unumeral.IntegerManager) *polynomialNormalizeInvocationReceipt {
	if c.normalization == nil || c.normalizationInvocation == 0 || c.normalizationInvocation != c.normalization.invocation || numericIntegerManager_normalizationAllocationRoots(m, c.normalization) == nil {
		return nil
	}
	return c.normalization
}
func numericIntegerModularManager_normalizationAllocationContext(m *unumeral.IntegerModularManager, r *polynomialNormalizeInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || m.Integers() == nil || numericIntegerManager_normalizationAllocationRoots(m.Integers(), r) == nil || r.manager.core.modular != m {
		return integerAllocationInvocationContext{}
	}
	return integerAllocationInvocationContext{normalization: r, normalizationInvocation: r.invocation}
}
func numericIntegerModularManager_copyWithNormalizeReceipt(m *unumeral.IntegerModularManager, out, value *unumeral.Int, r *polynomialNormalizeInvocationReceipt) error {
	if e := m.Check(); e != nil {
		return e
	}
	context := numericIntegerModularManager_normalizationAllocationContext(m, r)
	if e := m.Integers().WithAllocationContext((context).utility(m.Integers())).CopyFromOwnerTracked(out, m.Integers(), value, nil); e != nil {
		return e
	}
	return numericIntegerModularManager_normalizeWithSourceContext(m, out, context)
}
func numericIntegerModularManager_gcdManyIndexedWithNormalizeReceipt(m *unumeral.IntegerModularManager, as unumeral.IntegerArrayView, out *unumeral.Int, r *polynomialNormalizeInvocationReceipt) error {
	if e := m.Check(); e != nil {
		return e
	}
	if e := m.Integers().WithAllocationContext((numericIntegerModularManager_normalizationAllocationContext(m, r)).utility(m.Integers())).GcdManyIndexed(as, out); e != nil {
		return e
	}
	return m.AssertNormalized(out)
}
func numericIntegerModularManager_divWithNormalizeReceipt(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, r *polynomialNormalizeInvocationReceipt) error {
	return m.WithAllocationContext((numericIntegerModularManager_normalizationAllocationContext(m, r)).utility(m.Integers())).Div(a, b, out)
}
func (c integerAllocationInvocationContext) recordNormalizationRaw(m *unumeral.IntegerManager, r *unumeral.IntegerRawInvocationReceipt) {
	p := c.normalizationReceipt(m)
	if p == nil || !m.MatchesRawInvocation(r) {
		return
	}

	switch r.Kind() {
	case unumeral.IntegerRawGCD:
		p.frame.gcdRaw = *r
	case unumeral.IntegerRawDivision:
		p.frame.divisionRaw = *r
	case unumeral.IntegerRawExtendedGCD:
		p.frame.extendedRaw = *r
	}
}
func numericIntegerManager_normalizationNumericSourceCause(m *unumeral.IntegerManager, r *polynomialNormalizeInvocationReceipt) error {
	if r == nil || r.manager == nil || !r.manager.hasNormalizeInvocationHistory(r) || r.manager.core.integer != m {
		return nil
	}
	s := r.frame

	if e := m.RawInvocationSourceCause(&s.extendedRaw); e != nil {
		return e
	}
	if e := m.RawInvocationSourceCause(&s.divisionRaw); e != nil {
		return e
	}
	if e := m.RawInvocationSourceCause(&s.gcdRaw); e != nil {
		return e
	}
	return nil
}
func numericIntegerManager_cleanupNormalizationNumeric(m *unumeral.IntegerManager, r *polynomialNormalizeInvocationReceipt, abort bool) error {
	if r == nil || r.manager == nil || !r.manager.hasNormalizeInvocationHistory(r) || r.manager.core.integer != m {
		return nil
	}
	s := r.frame
	if s.executing {
		return ErrPolynomialPending
	}
	if e := r.manager.core.modular.CleanupPNormalizeInvocation(&s.normalize, abort); e != nil {
		return e
	}
	if e := m.CleanupRawInvocation(&s.extendedRaw, abort); e != nil {
		return e
	}
	if e := m.CleanupRawInvocation(&s.divisionRaw, abort); e != nil {
		return e
	}
	if e := m.CleanupRawInvocation(&s.gcdRaw, abort); e != nil {
		return e
	}

	if abort {

		return s.numeric.cleanup(m)
	}
	return nil
}

// Exact captured child identity is a refusal guard for owner destruction only;
// numerical acquisition remains exclusively the explicit passed context.
func (m *MultivariatePolynomialCore) normalizationPending() bool {
	if m.hasSOMNormalizeInvocationHistory(&m.somNormalizationOwner) && (m.somNormalizationOwner.frame.active || m.somNormalizationOwner.frame.finishing) {
		return true
	}
	r := &m.normalizationOwner
	if r.manager != nil && r.manager.hasNormalizeInvocationHistory(r) && r.manager.core == m && r.frame.active {
		return true
	}
	for _, b := range [4]*polynomialSOM{&m.som, &m.som2, &m.cheap, &m.cheap2} {
		if m.hasSOMNormalizeInvocationHistory(&b.normalizeScope.receipt) && b.normalizeScope.active {
			return true
		}
	}
	return false
}
