package polynomial

// Source: src/math/polynomial/polynomial.cpp:2757-2768;
// src/util/mpzzp.h:167-185,195-208; src/util/mpz.h:124-149.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerManager_scalarDivAllocationRoots(m *unumeral.IntegerManager, r *polynomialScalarDivInvocationReceipt) *polynomialNumericRoots {
	if r == nil || r.manager == nil || r.manager.self != r.manager || r.manager.integer != m || !r.manager.matchesScalarDivInvocation(r) || !r.frame.executing || r.manager.callbackDepth != 0 {
		return nil
	}
	return &r.frame.numeric
}
func numericIntegerModularManager_scalarDivAllocationContext(m *unumeral.IntegerModularManager, r *polynomialScalarDivInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || m.Integers() == nil || numericIntegerManager_scalarDivAllocationRoots(m.Integers(), r) == nil || r.manager.modular != m {
		return integerAllocationInvocationContext{}
	}
	return integerAllocationInvocationContext{scalarDiv: r}
}
func numericIntegerModularManager_copyWithScalarDivReceipt(m *unumeral.IntegerModularManager, out, value *unumeral.Int, r *polynomialScalarDivInvocationReceipt) error {
	if e := m.Check(); e != nil {
		return e
	}
	context := numericIntegerModularManager_scalarDivAllocationContext(m, r)
	if e := m.Integers().WithAllocationContext((context).utility(m.Integers())).CopyFromOwnerTracked(out, m.Integers(), value, nil); e != nil {
		return e
	}
	return numericIntegerModularManager_normalizeWithSourceContext(m, out, context)
}

func numericIntegerModularManager_divWithScalarDivReceipt(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, r *polynomialScalarDivInvocationReceipt) error {
	return m.WithAllocationContext((numericIntegerModularManager_scalarDivAllocationContext(m, r)).utility(m.Integers())).Div(a, b, out)
}
