package polynomial

// Source: algebraic_numbers.cpp display_root, retained by isolation TRACE.

// This receipt names the actual display invocation, including when its owner
// directly acknowledges it before another display is accepted.
type algebraicRootDisplayInvocationReceipt struct {
	owner      *AlgebraicRootCore
	invocation uint64
}

func (m *AlgebraicRootCore) matchesRootDisplayInvocation(r *algebraicRootDisplayInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.rootDisplay.invocation
}
func (m *AlgebraicRootCore) rootDisplayInvocationPending(r *algebraicRootDisplayInvocationReceipt) bool {
	return m.matchesRootDisplayInvocation(r) && m.rootDisplay.active
}
func (m *AlgebraicRootCore) cleanupRootDisplayInvocation(r *algebraicRootDisplayInvocationReceipt) error {
	if !m.matchesRootDisplayInvocation(r) {
		return nil
	}
	return m.cleanupRootDisplay()
}
