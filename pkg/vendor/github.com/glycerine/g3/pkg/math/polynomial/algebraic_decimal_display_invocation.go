package polynomial

// Source: algebraic_numbers.cpp:2731-2746; manager::display selects precision10.

type algebraicDecimalDisplayInvocationReceipt struct {
	owner      *AlgebraicRootCore
	invocation uint64
}

func (m *AlgebraicRootCore) matchesDecimalDisplayInvocation(r *algebraicDecimalDisplayInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.decimalDisplay.invocation
}
func (m *AlgebraicRootCore) decimalDisplayInvocationPending(r *algebraicDecimalDisplayInvocationReceipt) bool {
	return m.matchesDecimalDisplayInvocation(r) && m.decimalDisplay.pending(m)
}
func (m *AlgebraicRootCore) decimalDisplayInvocationSourceCause(r *algebraicDecimalDisplayInvocationReceipt) error {
	if !m.matchesDecimalDisplayInvocation(r) {
		return nil
	}
	s := &m.decimalDisplay
	if e := m.config.Rationals.DecimalInvocationSourceCause(&s.basic); e != nil {
		return e
	}
	return s.sourceStop
}
func (m *AlgebraicRootCore) cleanupDecimalDisplayInvocation(r *algebraicDecimalDisplayInvocationReceipt) error {
	if !m.matchesDecimalDisplayInvocation(r) {
		return nil
	}
	return m.cleanupDecimalDisplay()
}
