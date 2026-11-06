package numeral

// Source: src/util/mpbq.cpp refine_upper/refine_lower, retained by
// upolynomial.cpp:2849-2901. The raw midpoint survives a throwing body.

type BinaryRefineInvocationReceipt struct {
	owner      *BinaryRationalManager
	invocation uint64
}

func (m *BinaryRationalManager) RefineUpperWithReceipt(qm *RationalManager, q *Rational, l, u *BinaryRational, receipt *BinaryRefineInvocationReceipt) error {
	return m.refineWithReceipt(qm, q, l, u, true, receipt)
}
func (m *BinaryRationalManager) RefineLowerWithReceipt(qm *RationalManager, q *Rational, l, u *BinaryRational, receipt *BinaryRefineInvocationReceipt) error {
	return m.refineWithReceipt(qm, q, l, u, false, receipt)
}
func (m *BinaryRationalManager) matchesRefineInvocation(r *BinaryRefineInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.invocation != 0 && r.invocation == m.refineInvocation
}
func (m *BinaryRationalManager) RefineInvocationPending(r *BinaryRefineInvocationReceipt) bool {
	return m.matchesRefineInvocation(r) && (m.refinePending || m.refineImportedPending || m.refineDebugPending && m.refineDebugChildren.pending(m))
}
func (m *BinaryRationalManager) RefineInvocationSourceCause(r *BinaryRefineInvocationReceipt) error {
	if m.matchesRefineInvocation(r) {
		return m.refineSourceCause
	}
	return nil
}

// Completed debug/import locals precede the caller's older scoped bounds.
// The plain RAW midpoint is acknowledged separately after those bounds.
func (m *BinaryRationalManager) CleanupRefineInvocation(r *BinaryRefineInvocationReceipt, abort bool) error {
	if !m.matchesRefineInvocation(r) {
		return nil
	}
	if m.refineExecuting {
		return ErrBinaryRefinePending
	}
	if m.refineDebugPending {
		if e := m.refineDebugChildren.cleanup(m); e != nil {
			return e
		}
		m.refineDebugPending = false
	}
	if e := m.CleanupRefineImported(); e != nil {
		return e
	}
	if abort {
		return m.CleanupRefineMid()
	}
	return nil
}
