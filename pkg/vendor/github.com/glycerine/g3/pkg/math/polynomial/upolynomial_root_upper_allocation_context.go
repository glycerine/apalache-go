package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1906-1951;
// src/util/mpzzp.h:132,150,195-207,235.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerModularManager_rootUpperContextValid(m *unumeral.IntegerModularManager, c integerAllocationInvocationContext) bool {
	return m != nil && m.ValidOwner() && m.Integers() != nil && c.rootUpperBoundFrame(m.Integers()) != nil && c.rootUpperBound.owner.modular == m
}
func numericIntegerModularManager_copyWithRootUpperContext(m *unumeral.IntegerModularManager, out, value *unumeral.Int, c integerAllocationInvocationContext) error {
	if !numericIntegerModularManager_rootUpperContextValid(m, c) {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Copy(out, value)
}
func numericIntegerModularManager_absWithRootUpperContext(m *unumeral.IntegerModularManager, out *unumeral.Int, c integerAllocationInvocationContext) error {
	if !numericIntegerModularManager_rootUpperContextValid(m, c) {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Abs(out)
}
func numericIntegerModularManager_addWithRootUpperContext(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, c integerAllocationInvocationContext) error {
	if !numericIntegerModularManager_rootUpperContextValid(m, c) {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Add(a, b, out)
}
func numericIntegerModularManager_divWithRootUpperContext(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, c integerAllocationInvocationContext) error {
	if !numericIntegerModularManager_rootUpperContextValid(m, c) {
		return ErrUPolynomialOwner
	}
	return m.WithAllocationContext((c).utility(m.Integers())).Div(a, b, out)
}
