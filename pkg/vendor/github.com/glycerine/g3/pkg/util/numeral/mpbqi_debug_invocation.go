package numeral

// Source: basic_interval arithmetic's actual MPBQ_DEBUG children, retained by
// algebraic_numbers.cpp:1034 and its saved-interval destructors.

type BinaryIntervalDebugInvocationReceipt struct {
	owner      *BinaryIntervalManager
	invocation uint64
}

func (m *BinaryIntervalManager) AddWithDebugReceipt(a, b, out *BinaryInterval, r *BinaryIntervalDebugInvocationReceipt) error {
	return m.arithmeticWithDebugReceipt(a, b, out, 0, 0, r)
}
func (m *BinaryIntervalManager) SubWithDebugReceipt(a, b, out *BinaryInterval, r *BinaryIntervalDebugInvocationReceipt) error {
	return m.arithmeticWithDebugReceipt(a, b, out, 1, 0, r)
}
func (m *BinaryIntervalManager) MulWithDebugReceipt(a, b, out *BinaryInterval, r *BinaryIntervalDebugInvocationReceipt) error {
	return m.arithmeticWithDebugReceipt(a, b, out, 2, 0, r)
}
func (m *BinaryIntervalManager) PowerWithDebugReceipt(a *BinaryInterval, n uint32, out *BinaryInterval, r *BinaryIntervalDebugInvocationReceipt) error {
	return m.arithmeticWithDebugReceipt(a, nil, out, 3, n, r)
}
func (m *BinaryIntervalManager) arithmeticWithDebugReceipt(a, b, out *BinaryInterval, kind uint8, power uint32, r *BinaryIntervalDebugInvocationReceipt) (cause error) {
	if m != nil && m.self == m && (m.debugExecuting || m.debugPending) {
		return ErrBinaryDebugSnapshotPending
	}
	if kind == 3 {
		if e := m.pair(a, out); e != nil {
			return e
		}
	} else if e := m.triple(a, b, out); e != nil {
		return e
	}
	if m.debugInvocation == ^uint64(0) {
		return ErrBinaryDebugSnapshotPending
	}
	m.debugInvocation++
	m.debugSourceCause = nil
	if r != nil {
		*r = BinaryIntervalDebugInvocationReceipt{m, m.debugInvocation}
	}
	m.debugExecuting = true
	defer func() {
		b := m.bounds
		m.debugChildren.capture(b)
		m.debugPending = b.toOwnedRationalScope.pending || b.debugIntegerPending || b.debugArithmeticPending || len(b.debugSnapshotQueue) != 0
		m.debugSourceCause = b.toOwnedRationalScope.sourceCause
		if m.debugSourceCause == nil && b.debugArithmeticPending {
			m.debugSourceCause = b.debugArithmeticCall.SourceReady()
		}
		if m.debugSourceCause == nil && (b.debugIntegerPending || len(b.debugSnapshotQueue) != 0) {
			m.debugSourceCause = cause
		}
		m.debugExecuting = false
	}()
	switch kind {
	case 0:
		return m.Add(a, b, out)
	case 1:
		return m.Sub(a, b, out)
	case 2:
		return m.Mul(a, b, out)
	default:
		return m.Power(a, power, out)
	}
}
func (m *BinaryIntervalManager) matchesDebugInvocation(r *BinaryIntervalDebugInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.debugInvocation
}
func (m *BinaryIntervalManager) DebugInvocationPending(r *BinaryIntervalDebugInvocationReceipt) bool {
	return m.matchesDebugInvocation(r) && m.debugPending && m.debugChildren.pending(m.bounds)
}
func (m *BinaryIntervalManager) DebugInvocationSourceCause(r *BinaryIntervalDebugInvocationReceipt) error {
	if m.matchesDebugInvocation(r) {
		return m.debugSourceCause
	}
	return nil
}
func (m *BinaryIntervalManager) CleanupDebugInvocation(r *BinaryIntervalDebugInvocationReceipt) error {
	if !m.matchesDebugInvocation(r) {
		return nil
	}
	if m.debugExecuting {
		return ErrBinaryDebugSnapshotPending
	}
	if !m.debugPending {
		return nil
	}
	if e := m.debugChildren.cleanup(m.bounds); e != nil {
		return e
	}
	m.debugPending = false
	return nil
}
