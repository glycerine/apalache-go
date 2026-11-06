package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1213-1240;
// src/math/polynomial/upolynomial_factorization_int.h:345-413;
// src/util/mpz.h:124-149; src/util/mpzzp.h:150-163.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c integerAllocationInvocationContext) trialFrame(z *unumeral.IntegerManager) *upolyRecombinationTrialScope {
	q := c.trial
	if z == nil || !z.ValidOwner() || q == nil || !q.matches() || !q.active || !q.executing || !q.constructed || c.trialInvocation == 0 || c.trialInvocation != q.invocation || q.owner.service.scope != q.owner || q.owner.manager.config.Integer != z {
		return nil
	}
	for _, m := range q.managers() {
		if m == nil || m.self != m || m.config.Integer != z || m.callbackDepth != 0 {
			return nil
		}
	}
	m := c.trialManager
	if m != q.owner.manager && m != q.factors.manager {
		return nil
	}
	return q
}
func numericIntegerModularManager_trialAllocationContext(m *unumeral.IntegerModularManager, q *upolyRecombinationTrialScope) integerAllocationInvocationContext {
	if m == nil || !m.ValidOwner() || m.Integers() == nil || q == nil || !q.matches() {
		return integerAllocationInvocationContext{}
	}
	for _, owner := range q.managers() {
		if owner != nil && owner.modular == m {
			c := integerAllocationInvocationContext{trial: q, trialInvocation: q.invocation, trialManager: owner}
			if c.trialFrame(m.Integers()) != nil {
				return c
			}
		}
	}
	return integerAllocationInvocationContext{}
}
func (q *upolyRecombinationTrialScope) normalizeReceipt(m *unumeral.IntegerModularManager) *unumeral.IntegerNormalizeInvocationReceipt {
	for i, owner := range q.managers() {
		if owner.modular == m {
			return &q.normalize[i]
		}
	}
	return nil
}
