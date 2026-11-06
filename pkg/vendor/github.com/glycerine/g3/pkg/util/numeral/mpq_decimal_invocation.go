package numeral

// Source: mpq.cpp:156-185, retained by algebraic decimal display.

type RationalDecimalInvocationReceipt struct {
	owner      *RationalManager
	invocation uint64
}

func (m *RationalManager) MatchesDecimalInvocation(r *RationalDecimalInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.invocation != 0 && m.decimalStream.valid() && r.invocation == m.decimalStream.invocation
}
func (m *RationalManager) DecimalInvocationPending(r *RationalDecimalInvocationReceipt) bool {
	return m.MatchesDecimalInvocation(r) && m.decimalStream.active
}
func (m *RationalManager) DecimalInvocationSourceCause(r *RationalDecimalInvocationReceipt) error {
	if m.MatchesDecimalInvocation(r) {
		return m.decimalStream.sourceCause
	}
	return nil
}
func (m *RationalManager) CleanupDecimalInvocation(r *RationalDecimalInvocationReceipt) error {
	if !m.MatchesDecimalInvocation(r) || !m.decimalStream.active {
		return nil
	}
	return m.CleanupDecimalStream()
}
