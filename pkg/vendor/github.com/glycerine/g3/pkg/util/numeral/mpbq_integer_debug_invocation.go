package numeral

// Source: mpbq.cpp add/sub(mpz), retained by algebraic root_core bounds.

type BinaryIntegerDebugInvocationReceipt struct {
	owner      *BinaryRationalManager
	invocation uint64
}

func (m *BinaryRationalManager) AddIntegerWithDebugReceipt(a *BinaryRational, b *Int, out *BinaryRational, r *BinaryIntegerDebugInvocationReceipt) error {
	return m.addSubIntegerWithDebugReceipt(a, b, out, false, r)
}
func (m *BinaryRationalManager) SubIntegerWithDebugReceipt(a *BinaryRational, b *Int, out *BinaryRational, r *BinaryIntegerDebugInvocationReceipt) error {
	return m.addSubIntegerWithDebugReceipt(a, b, out, true, r)
}
func (m *BinaryRationalManager) addSubIntegerWithDebugReceipt(a *BinaryRational, b *Int, out *BinaryRational, sub bool, r *BinaryIntegerDebugInvocationReceipt) (cause error) {
	if m != nil && m.self == m && (m.integerDebugExecuting || m.integerDebugPending) {
		return ErrBinaryDebugSnapshotPending
	}
	if e := m.pair(a, out); e != nil {
		return e
	}
	if e := m.Z.check(b); e != nil {
		return e
	}
	if m.integerDebugInvocation == ^uint64(0) {
		return ErrBinaryDebugSnapshotPending
	}
	m.integerDebugInvocation++
	m.integerDebugSourceCause = nil
	m.integerDebugExecuting = true
	if r != nil {
		*r = BinaryIntegerDebugInvocationReceipt{m, m.integerDebugInvocation}
	}
	defer func() {
		m.integerDebugChildren.capture(m)
		m.integerDebugPending = m.integerDebugChildren.pending(m)
		m.integerDebugSourceCause = m.integerDebugChildren.sourceCause(m, cause)
		m.integerDebugExecuting = false
	}()
	return m.addSubInteger(a, b, out, sub)
}
func (m *BinaryRationalManager) matchesIntegerDebugInvocation(r *BinaryIntegerDebugInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.integerDebugInvocation
}
func (m *BinaryRationalManager) IntegerDebugInvocationPending(r *BinaryIntegerDebugInvocationReceipt) bool {
	return m.matchesIntegerDebugInvocation(r) && m.integerDebugPending && m.integerDebugChildren.pending(m)
}
func (m *BinaryRationalManager) IntegerDebugInvocationSourceCause(r *BinaryIntegerDebugInvocationReceipt) error {
	if m.matchesIntegerDebugInvocation(r) {
		return m.integerDebugSourceCause
	}
	return nil
}
func (m *BinaryRationalManager) CleanupIntegerDebugInvocation(r *BinaryIntegerDebugInvocationReceipt) error {
	if !m.matchesIntegerDebugInvocation(r) {
		return nil
	}
	if m.integerDebugExecuting {
		return ErrBinaryDebugSnapshotPending
	}
	if m.integerDebugPending {
		if e := m.integerDebugChildren.cleanup(m); e != nil {
			return e
		}
		m.integerDebugPending = false
	}
	return nil
}
