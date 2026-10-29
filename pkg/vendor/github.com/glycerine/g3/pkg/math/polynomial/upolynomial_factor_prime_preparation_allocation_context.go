package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1058-1094,1153-1175;
// src/util/mpz.h:680-707; src/util/mpz.cpp:560-843;
// src/util/mpzzp.h:122-123,211-216,229-235.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c integerAllocationInvocationContext) primePreparationFrame(z *unumeral.IntegerManager) *upolyFactorPrimePreparationScope {
	q := c.primePreparation
	if z == nil || !z.ValidOwner() || q == nil || !q.matches() || !q.active || (!q.executing && !q.sourceExecuting) || q.owner.constructed < 2 || c.primePreparationInvocation == 0 || c.primePreparationInvocation != q.invocation || q.owner.service.scope != q.owner || q.owner.manager.config.Integer != z {
		return nil
	}
	for _, m := range q.managers() {
		if m == nil {
			continue
		}
		if m.self != m || m.config.Integer != z || m.callbackDepth != 0 {
			return nil
		}
	}
	m := c.primePreparationManager
	if m != q.owner.manager && m != q.related && m != q.lift {
		return nil
	}
	return q
}
func numericIntegerModularManager_primePreparationAllocationContext(m *unumeral.IntegerModularManager, q *upolyFactorPrimePreparationScope) integerAllocationInvocationContext {
	if m == nil || !m.ValidOwner() || m.Integers() == nil || q == nil || !q.matches() {
		return integerAllocationInvocationContext{}
	}
	for _, owner := range q.managers() {
		if owner != nil && owner.modular == m {
			c := integerAllocationInvocationContext{primePreparation: q, primePreparationInvocation: q.invocation, primePreparationManager: owner}
			if c.primePreparationFrame(m.Integers()) != nil {
				return c
			}
		}
	}
	return integerAllocationInvocationContext{}
}
func (q *upolyFactorPrimePreparationScope) normalizeReceipt(m *unumeral.IntegerModularManager) *unumeral.IntegerNormalizeInvocationReceipt {
	for i, owner := range q.managers() {
		if owner != nil && owner.modular == m {
			return &q.normalize[i]
		}
	}
	return nil
}

func (c integerAllocationInvocationContext) recordPrimePreparationRaw(z *unumeral.IntegerManager, r *unumeral.IntegerRawInvocationReceipt) {
	q := c.primePreparationFrame(z)
	if q == nil || !z.MatchesRawInvocation(r) {
		return
	}
	switch r.Kind() {
	case unumeral.IntegerRawDivision:
		q.divisionRaw = *r
	case unumeral.IntegerRawExtendedGCD:
		q.extendedRaw = *r
	case unumeral.IntegerRawGCD:
		q.gcdRaw = *r
	}
}
