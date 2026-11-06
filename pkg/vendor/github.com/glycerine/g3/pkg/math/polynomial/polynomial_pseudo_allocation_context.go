package polynomial

// Source: src/math/polynomial/polynomial.cpp:4939-4949,4989-5010;
// src/util/mpzzp.h:58-68,131; src/util/mpz.h:124-149.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerManager_pseudoAllocationRoots(m *unumeral.IntegerManager, r *polynomialPseudoDivisionInvocationReceipt) *polynomialNumericRoots {
	if m == nil || !m.ValidOwner() || r == nil || r.manager == nil || r.manager.integer != m || !r.manager.matchesPseudoInvocation(r) || !r.frame.executing || r.manager.callbackDepth != 0 || r.frame.constructed&(uint16(1)<<pseudoNumeral) == 0 {
		return nil
	}
	return &r.frame.numeric
}
func numericIntegerManager_pseudoContextRoots(m *unumeral.IntegerManager, c integerAllocationInvocationContext) *polynomialNumericRoots {
	if c.pseudo == nil || c.pseudoInvocation == 0 || c.pseudoInvocation != c.pseudo.invocation {
		return nil
	}
	return numericIntegerManager_pseudoAllocationRoots(m, c.pseudo)
}
func numericIntegerModularManager_pseudoAllocationContext(m *unumeral.IntegerModularManager, r *polynomialPseudoDivisionInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || m.Integers() == nil || numericIntegerManager_pseudoAllocationRoots(m.Integers(), r) == nil || r.manager.modular != m {
		return integerAllocationInvocationContext{}
	}
	return integerAllocationInvocationContext{pseudo: r, pseudoInvocation: r.invocation}
}
func numericIntegerModularManager_copyWithPseudoReceipt(m *unumeral.IntegerModularManager, out, value *unumeral.Int, r *polynomialPseudoDivisionInvocationReceipt) error {
	if e := m.Check(); e != nil {
		return e
	}
	context := numericIntegerModularManager_pseudoAllocationContext(m, r)
	if e := m.Integers().WithAllocationContext((context).utility(m.Integers())).CopyFromOwnerTracked(out, m.Integers(), value, nil); e != nil {
		return e
	}
	return numericIntegerModularManager_normalizeWithSourceContext(m, out, context)
}
func numericIntegerModularManager_negWithPseudoReceipt(m *unumeral.IntegerModularManager, out *unumeral.Int, r *polynomialPseudoDivisionInvocationReceipt) error {
	if e := m.Check(); e != nil {
		return e
	}
	context := numericIntegerModularManager_pseudoAllocationContext(m, r)
	if e := m.Integers().WithAllocationContext((context).utility(m.Integers())).Neg(out); e != nil {
		return e
	}
	return numericIntegerModularManager_normalizeWithSourceContext(m, out, context)
}
