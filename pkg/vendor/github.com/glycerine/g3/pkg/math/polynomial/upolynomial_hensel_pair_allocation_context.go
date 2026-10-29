package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:583-674;
// src/util/mpz.h:124-149; src/util/mpzzp.h:154,230-233.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c integerAllocationInvocationContext) henselPairReceipt(z *unumeral.IntegerManager) *upolyHenselPairInvocationReceipt {
	r := c.henselPair
	if z == nil || !z.ValidOwner() || r == nil || r.service == nil || !r.service.matchesHenselPairInvocation(r) || r.service.manager.config.Integer != z || c.henselPairInvocation == 0 || c.henselPairInvocation != r.invocation || !r.frame.executing || r.frame.constructed == 0 || r.service.manager.callbackDepth != 0 {
		return nil
	}
	m := c.henselManager
	if m == nil || m.self != m || m.config.Integer != z || m.callbackDepth != 0 || !r.frame.ownsNumericManager(m) {
		return nil
	}
	if r.frame.rm != nil && r.frame.rm.callbackDepth != 0 {
		return nil
	}
	for _, owner := range r.frame.numericOwners() {
		if owner != nil && owner.callbackDepth != 0 {
			return nil
		}
	}
	return r
}
func numericIntegerModularManager_henselPairAllocationContext(m *unumeral.IntegerModularManager, r *upolyHenselPairInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || !m.ValidOwner() || m.Integers() == nil || r == nil || r.service == nil {
		return integerAllocationInvocationContext{}
	}
	var owner *UPolynomialManager
	if r.frame != nil {
		for _, candidate := range r.frame.numericOwners() {
			if candidate != nil && candidate.modular == m {
				owner = candidate
				break
			}
		}
	}
	c := integerAllocationInvocationContext{henselPair: r, henselPairInvocation: r.invocation, henselManager: owner}
	if c.henselPairReceipt(m.Integers()) == nil {
		return integerAllocationInvocationContext{}
	}
	return c
}
func (h *upolyHenselPairScope) numericOwners() [4]*UPolynomialManager {
	return [4]*UPolynomialManager{h.service.manager, h.rm, h.check.brCore, h.check.bCore}
}
func (h *upolyHenselPairScope) numericManagers() [4]*unumeral.IntegerModularManager {
	var out [4]*unumeral.IntegerModularManager
	for i, m := range h.numericOwners() {
		if m != nil {
			out[i] = m.modular
		}
	}
	return out
}
func (h *upolyHenselPairScope) ownsNumericManager(m *UPolynomialManager) bool {
	for _, owner := range h.numericOwners() {
		if owner == m {
			return true
		}
	}
	return false
}
func (h *upolyHenselPairScope) normalizeReceipt(m *unumeral.IntegerModularManager) *unumeral.IntegerNormalizeInvocationReceipt {
	for i, owner := range h.numericManagers() {
		if owner == m {
			return &h.normalize[i]
		}
	}
	return nil
}
func numericIntegerManager_bindHenselRaw(z *unumeral.IntegerManager, c integerAllocationInvocationContext, r *unumeral.IntegerRawInvocationReceipt, division bool) {
	p := c.henselPairReceipt(z)
	if p == nil {
		return
	}
	if division {
		p.frame.divisionRaw = *r
	} else {
		p.frame.extendedRaw = *r
	}

}

func henselAddMulChild(r *upolyHenselPairInvocationReceipt) *unumeral.IntegerAddMulInvocationReceipt {
	if r == nil {
		return nil
	}
	return &r.frame.addMul
}

func numericIntegerModularManager_dividesWithHenselContext(m *unumeral.IntegerModularManager, a, b *unumeral.Int, c integerAllocationInvocationContext) (bool, error) {
	return m.WithAllocationContext((c).utility(m.Integers())).Divides(a, b)
}
