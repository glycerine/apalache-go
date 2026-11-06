package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1238-1292;
// src/math/polynomial/upolynomial.cpp:56-63,213-232,609-651;
// src/util/mpz.h:124-149; src/util/mpzzp.h:150-165.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c integerAllocationInvocationContext) recombinationFrame(z *unumeral.IntegerManager) *upolyRecombinationStepScope {
	q := c.recombination
	if z == nil || !z.ValidOwner() || q == nil || !q.matches() || !q.active || !q.executing || c.recombinationInvocation == 0 || c.recombinationInvocation != q.invocation || q.owner.service.scope != q.owner || q.owner.manager.config.Integer != z {
		return nil
	}
	for _, m := range q.managers() {
		if m == nil || m.self != m || m.config.Integer != z || m.callbackDepth != 0 {
			return nil
		}
	}
	m := c.recombinationManager
	if m != q.owner.manager && m != q.related {
		return nil
	}
	return q
}
func numericIntegerModularManager_recombinationAllocationContext(m *unumeral.IntegerModularManager, q *upolyRecombinationStepScope) integerAllocationInvocationContext {
	if m == nil || !m.ValidOwner() || m.Integers() == nil || q == nil || !q.matches() {
		return integerAllocationInvocationContext{}
	}
	for _, owner := range q.managers() {
		if owner != nil && owner.modular == m {
			c := integerAllocationInvocationContext{recombination: q, recombinationInvocation: q.invocation, recombinationManager: owner}
			if c.recombinationFrame(m.Integers()) != nil {
				return c
			}
		}
	}
	return integerAllocationInvocationContext{}
}
func (q *upolyRecombinationStepScope) normalizeReceipt(m *unumeral.IntegerModularManager) *unumeral.IntegerNormalizeInvocationReceipt {
	for i, owner := range q.managers() {
		if owner.modular == m {
			return &q.normalize[i]
		}
	}
	return nil
}

func (c integerAllocationInvocationContext) recordRecombinationRaw(z *unumeral.IntegerManager, r *unumeral.IntegerRawInvocationReceipt) {
	q := c.recombinationFrame(z)
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

func (m *UPolynomialManager) mulScalarStateWithRecombination(s *upolyVectorState, b *unumeral.Int, q *upolyRecombinationStepScope) error {
	context := numericIntegerModularManager_recombinationAllocationContext(m.modular, q)
	if context.recombinationFrame(m.config.Integer) == nil {
		return ErrUPolynomialOwner
	}
	zero, e := m.modular.IsZero(b)
	if e != nil {
		return e
	}
	if zero {
		return m.reset(s)
	}
	return m.mulScalarViewWithSourceContext(m.stateView(s), b, context)
}
