package numeral

// Source: src/util/mpbq.cpp set(mpq), including its reached debug snapshots.

type BinaryConvertInvocationReceipt struct {
	owner      *BinaryRationalManager
	invocation uint64
}

func (m *BinaryRationalManager) matchesConvertInvocation(r *BinaryConvertInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.convertInvocation
}
func (m *BinaryRationalManager) ConvertInvocationPending(r *BinaryConvertInvocationReceipt) bool {
	return m.matchesConvertInvocation(r) && (m.convertImportedPending || m.convertDebugPending && m.convertDebugChildren.pending(m))
}
func (m *BinaryRationalManager) ConvertInvocationSourceCause(r *BinaryConvertInvocationReceipt) error {
	if m.matchesConvertInvocation(r) {
		return m.convertSourceCause
	}
	return nil
}
func (m *BinaryRationalManager) CleanupConvertInvocation(r *BinaryConvertInvocationReceipt) error {
	if !m.matchesConvertInvocation(r) {
		return nil
	}
	if m.convertExecuting {
		return ErrBinaryConvertPending
	}
	if m.convertDebugPending {
		if e := m.convertDebugChildren.cleanup(m); e != nil {
			return e
		}
		m.convertDebugPending = false
	}
	return m.CleanupConvertImported()
}
