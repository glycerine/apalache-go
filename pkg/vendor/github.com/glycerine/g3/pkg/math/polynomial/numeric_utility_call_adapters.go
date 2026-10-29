package polynomial

// Source: src/math/polynomial numeric invocation adapters.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerManager_addMulWithSourceReceipt(m *unumeral.IntegerManager, a, b, c, d *unumeral.Int, receipt *unumeral.IntegerAddMulInvocationReceipt, arithmetic *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(((integerAllocationInvocationContext{}).withArithmeticInvocation(arithmetic)).utility(m)).AddMulTracked(a, b, c, d, receipt)
}

func numericRationalManager_setIntegerWithArithmeticReceipt(m *unumeral.RationalManager, a *unumeral.Rational, n *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(((integerAllocationInvocationContext{}).withArithmeticInvocation(receipt)).utility(m.Integers())).SetInteger(a, n)
}

func numericIntegerModularManager_addWithArithmeticReceipt(m *unumeral.IntegerModularManager, a, b, c *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext((numericIntegerModularManager_arithmeticAllocationContext(m, receipt)).utility(m.Integers())).Add(a, b, c)
}

func numericIntegerModularManager_mulWithArithmeticReceipt(m *unumeral.IntegerModularManager, a, b, c *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext((numericIntegerModularManager_arithmeticAllocationContext(m, receipt)).utility(m.Integers())).Mul(a, b, c)
}

func numericIntegerModularManager_addMulWithArithmeticReceipt(m *unumeral.IntegerModularManager, a, b, c, d *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	receipt = numericIntegerModularManager_arithmeticReceiptForReceiver(m, receipt)
	child := numericIntegerModularManager_arithmeticAddMulReceipt(m, receipt)
	e := m.WithAllocationContext((numericIntegerModularManager_arithmeticAllocationContext(m, receipt)).utility(m.Integers())).AddMulTracked(a, b, c, d, child)
	if child != nil && receipt.frame.sourceCause == nil {
		receipt.frame.sourceCause = m.Integers().AddMulInvocationSourceCause(child)
	}
	return e
}

func numericIntegerModularManager_setInt64WithArithmeticReceipt(m *unumeral.IntegerModularManager, a *unumeral.Int, v int64, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext((numericIntegerModularManager_arithmeticAllocationContext(m, receipt)).utility(m.Integers())).SetInt64(a, v)
}

func numericIntegerModularManager_copyWithArithmeticReceipt(m *unumeral.IntegerModularManager, a, b *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext((numericIntegerModularManager_arithmeticAllocationContext(m, receipt)).utility(m.Integers())).Copy(a, b)
}
