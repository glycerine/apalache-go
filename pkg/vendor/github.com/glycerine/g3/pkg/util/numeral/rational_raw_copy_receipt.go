package numeral

// CopyWithAllocationReceipt executes the same raw mpq numerator/denominator
// copies as Copy and records only an actually accepted integer allocation.
// CleanupCopyAllocationInvocation on Integers acknowledges that exact child.
func (m *RationalManager) CopyWithAllocationReceipt(dst, src *Rational, receipt *IntegerCopyAllocationReceipt) error {
	return m.copyWithAllocationReceipt(dst, src, integerAllocationInvocationContext{}, receipt)
}
