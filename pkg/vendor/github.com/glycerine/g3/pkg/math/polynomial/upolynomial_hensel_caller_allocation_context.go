package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:703-801,845-933;
// src/util/mpz.h:124-149; src/util/mpzzp.h:46-56,154,230-233.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (q *upolyHenselCallerScope) numericOwners() [4]*UPolynomialManager {
	if q == nil || q.owner == nil {
		return [4]*UPolynomialManager{}
	}
	return [4]*UPolynomialManager{q.owner.manager, q.related, q.prime, q.lift}
}
func (q *upolyHenselCallerScope) ownsNumericManager(m *UPolynomialManager) bool {
	for _, o := range q.numericOwners() {
		if o == m {
			return true
		}
	}
	return false
}
func (c integerAllocationInvocationContext) henselCallerFrame(z *unumeral.IntegerManager) *upolyHenselCallerScope {
	q := c.henselCaller
	if z == nil || !z.ValidOwner() || q == nil || !q.matches() || q.invocation == 0 || c.henselCallerInvocation != q.invocation || !q.executing || q.owner.manager.config.Integer != z {
		return nil
	}
	m := c.henselCallerManager
	if m == nil || m.self != m || !q.ownsNumericManager(m) || m.config.Integer != z {
		return nil
	}
	for _, o := range q.numericOwners() {
		if o != nil && o.callbackDepth != 0 {
			return nil
		}
	}
	return q
}
func numericIntegerModularManager_henselCallerAllocationContext(m *unumeral.IntegerModularManager, q *upolyHenselCallerScope) integerAllocationInvocationContext {
	if m == nil || !m.ValidOwner() || m.Integers() == nil || q == nil || q.owner == nil {
		return integerAllocationInvocationContext{}
	}
	for _, o := range q.numericOwners() {
		if o != nil && o.modular == m {
			c := integerAllocationInvocationContext{henselCaller: q, henselCallerInvocation: q.invocation, henselCallerManager: o}
			if c.henselCallerFrame(m.Integers()) != nil {
				return c
			}
		}
	}
	return integerAllocationInvocationContext{}
}
func (q *upolyHenselCallerScope) normalizeReceipt(m *unumeral.IntegerModularManager) *unumeral.IntegerNormalizeInvocationReceipt {
	for i, o := range q.numericOwners() {
		if o != nil && o.modular == m {
			return &q.normalize[i]
		}
	}
	return nil
}
func numericIntegerManager_bindHenselCallerRaw(z *unumeral.IntegerManager, c integerAllocationInvocationContext, r *unumeral.IntegerRawInvocationReceipt, division bool) {
	if q := c.henselCallerFrame(z); q != nil {
		if division {
			q.divisionRaw = *r
		} else {
			q.extendedRaw = *r
		}

	}
}

func (q *upolyHenselCallerScope) numericManagers() [4]*unumeral.IntegerModularManager {
	var out [4]*unumeral.IntegerModularManager
	for i, m := range q.numericOwners() {
		if m != nil {
			out[i] = m.modular
		}
	}
	return out
}
