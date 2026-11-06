package polynomial

import "io"

func (r *upolyDisplayInvocationReceipt) sourceBlocker() error {
	if !r.owner.displayReceiptPending(r) {
		return nil
	}
	if e := r.owner.displayReceiptSourceCause(r); e != nil {
		return e
	}
	return ErrUPolynomialCleanupPending
}
func (r *upolyDisplayInvocationReceipt) cleanupOwned() error {
	if r.owner == nil {
		return nil
	}
	m := r.owner
	if e := m.cleanupDisplayReceipt(r); e != nil {
		return e
	}
	if m.pendingExactDivTrace == r && !m.displayReceiptPending(r) {
		m.pendingExactDivTrace = nil
	}
	if m.pendingModGCDTrace == r && !m.displayReceiptPending(r) {
		m.pendingModGCDTrace = nil
	}
	if m.pendingEuclidTrace == r && !m.displayReceiptPending(r) {
		m.pendingEuclidTrace = nil
	}
	if m.pendingFactorCoreTrace == r && !m.displayReceiptPending(r) {
		m.pendingFactorCoreTrace = nil
	}
	return nil
}
func (c integerAllocationInvocationContext) exactDivTraceSlot(m *UPolynomialManager) *upolyDisplayInvocationReceipt {
	if q := c.squareFreeCallFrame(m.config.Integer); q != nil {
		return &q.exactTrace
	}
	if q := c.factorFrontendFrame(m.config.Integer); q != nil && q.service.manager == m {
		return &q.exactTrace
	}
	if q := c.recombinationFrame(m.config.Integer); q != nil && q.owner.manager == m {
		return &q.exactTrace
	}
	return &m.exactDivTrace
}

// Source: upolynomial.cpp:628, including the final empty-remainder iteration.
// Both actual polynomial displays evaluate after a sticky ostream failure.
func (m *UPolynomialManager) traceExactDivLoop(context integerAllocationInvocationContext) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	slot := context.exactDivTraceSlot(m)
	var computation error
	_ = tags.EmitConditional("upolynomial", "exact_div", "src/math/polynomial/upolynomial.cpp", 628, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		_ = linearWriteText(out, "exact_div loop...\n")
		for _, v := range []*upolyVectorState{&m.div1, &m.exactDiv} {
			var issued upolyDisplayInvocationReceipt
			e := m.displayIntoWithReceipt(m.stateView(v), factorizationTraceStream{out}, "x", false, &issued)
			if m.matchesDisplayReceipt(&issued) {
				*slot = issued
				if m.displayReceiptPending(slot) {
					m.pendingExactDivTrace = slot
				}
			}
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, "\n")
		}
		return nil
	})
	return computation
}

// CleanupExactDivisionTrace owns only the default public exact-division call's
// accepted display. Factorization frames clean their own separate receipts.
func (m *UPolynomialManager) CleanupExactDivisionTrace() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if m.factorTrace != nil && m.factorTrace.executing || m.displayExecuting() {
		return ErrUPolynomialCleanupPending
	}
	return m.exactDivTrace.cleanupOwned()
}
