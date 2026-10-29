package polynomial

// Source: src/math/polynomial/upolynomial.cpp:213-232,3042-3118;
// src/util/mpz.cpp:560-843; src/util/mpzzp.h:229-235.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c integerAllocationInvocationContext) factorFrontendFrame(z *unumeral.IntegerManager) *upolyFactorFrontendScope {
	q := c.factorFrontend
	if z == nil || !z.ValidOwner() || q == nil || q.self != q || q.service == nil || q.service.self != q.service || q != &q.service.frontend || !q.active || (!q.executing && !q.sourceExecuting && !q.ownExecuting) || q.constructed < 2 || c.factorFrontendInvocation == 0 || c.factorFrontendInvocation != q.invocation {
		return nil
	}
	m := q.service.manager
	if m == nil || m.self != m || m.config.Integer != z || m.callbackDepth != 0 || q.service.closed {
		return nil
	}
	return q
}
func numericIntegerModularManager_factorFrontendAllocationContext(m *unumeral.IntegerModularManager, q *upolyFactorFrontendScope) integerAllocationInvocationContext {
	if m == nil || !m.ValidOwner() || m.Integers() == nil || q == nil || q.service == nil || q.service.manager == nil || q.service.manager.modular != m {
		return integerAllocationInvocationContext{}
	}
	c := integerAllocationInvocationContext{factorFrontend: q, factorFrontendInvocation: q.invocation}
	if c.factorFrontendFrame(m.Integers()) == nil {
		return integerAllocationInvocationContext{}
	}
	return c
}
func (c integerAllocationInvocationContext) recordFactorFrontendRaw(z *unumeral.IntegerManager, r *unumeral.IntegerRawInvocationReceipt) {
	q := c.factorFrontendFrame(z)
	if q == nil || !z.MatchesRawInvocation(r) {
		return
	}
	switch r.Kind() {
	case unumeral.IntegerRawGCD:
		q.gcdRaw = *r
	case unumeral.IntegerRawDivision:
		q.divisionRaw = *r
	case unumeral.IntegerRawExtendedGCD:
		q.extendedRaw = *r
	}
}

// History validates exact current transport identity independently of whether
// an older caller NUM has already been consumed by explicit owning cleanup.
func (c integerAllocationInvocationContext) factorFrontendHistoryFrame(z *unumeral.IntegerManager) *upolyFactorFrontendScope {
	q := c.factorFrontend
	if z == nil || !z.ValidOwner() || q == nil || !q.actual() || !q.active || q.service.manager.config.Integer != z || c.factorFrontendInvocation == 0 || c.factorFrontendInvocation != q.invocation || !q.service.matchesFactorFrontendHistory(&q.receipt) {
		return nil
	}
	return q
}
