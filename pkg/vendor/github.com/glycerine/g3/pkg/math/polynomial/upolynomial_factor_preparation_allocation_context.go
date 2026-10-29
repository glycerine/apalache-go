package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1030-1050;
// src/util/mpz.h:124-149; src/util/mpzzp.h:131-132,229-235.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c integerAllocationInvocationContext) preparationFrame(z *unumeral.IntegerManager) *upolyFactorPreparationScope {
	q := c.preparation
	if z == nil || !z.ValidOwner() || q == nil || !q.matches(&q.receipt) || !q.active || !q.executing || c.preparationInvocation == 0 || c.preparationInvocation != q.invocation {
		return nil
	}
	m := q.service.manager
	if m.config.Integer != z || m.callbackDepth != 0 || q.service.closed {
		return nil
	}
	if q.parent != nil && q.service.scope != q.parent {
		return nil
	}
	return q
}
func numericIntegerModularManager_preparationAllocationContext(m *unumeral.IntegerModularManager, q *upolyFactorPreparationScope) integerAllocationInvocationContext {
	if m == nil || !m.ValidOwner() || m.Integers() == nil || q == nil || !q.matches(&q.receipt) || q.service.manager.modular != m {
		return integerAllocationInvocationContext{}
	}
	c := integerAllocationInvocationContext{preparation: q, preparationInvocation: q.invocation}
	if c.preparationFrame(m.Integers()) == nil {
		return integerAllocationInvocationContext{}
	}
	return c
}
func (c integerAllocationInvocationContext) recordPreparationRaw(z *unumeral.IntegerManager, r *unumeral.IntegerRawInvocationReceipt) {
	q := c.preparationFrame(z)
	if q != nil && z.MatchesRawInvocation(r) && r.Kind() == unumeral.IntegerRawGCD {
		q.gcdRaw = *r
	}
}
