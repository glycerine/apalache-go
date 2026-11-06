package numeral

// Source: mpbq.cpp:426-458; actual retained display caller cleanup identity.

type BinaryDisplayInvocationReceipt struct {
	manager    *BinaryRationalManager
	invocation uint64
}

func (m *BinaryRationalManager) matchesDisplayInvocation(r *BinaryDisplayInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.invocation != 0 && r.invocation == m.Display.invocation
}
func (m *BinaryRationalManager) DisplayInvocationPending(r *BinaryDisplayInvocationReceipt) bool {
	return m.matchesDisplayInvocation(r) && m.Display.active
}
func (m *BinaryRationalManager) DisplayInvocationSourceCause(r *BinaryDisplayInvocationReceipt) error {
	if m.matchesDisplayInvocation(r) && m.Display.sourceEnd {
		return m.Display.cleanup
	}
	return nil
}
func (m *BinaryRationalManager) CleanupDisplayInvocation(r *BinaryDisplayInvocationReceipt) error {
	if !m.DisplayInvocationPending(r) {
		return nil
	}
	return m.CleanupDisplay()
}
