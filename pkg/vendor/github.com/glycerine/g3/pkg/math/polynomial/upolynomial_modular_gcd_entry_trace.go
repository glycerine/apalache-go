package polynomial

import "io"

// Source: upolynomial.cpp:726, before assertions or any scoped NUM.
func (m *UPolynomialManager) traceModGCDEntry(a, b UPolynomialView, context integerAllocationInvocationContext) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	slot := &m.modGCDTrace
	if q := context.squareFreeCallFrame(m.config.Integer); q != nil {
		slot = &q.modGCDTrace
	}
	if f := context.factorFrontendFrame(m.config.Integer); f != nil && f.service.manager == m {
		slot = &f.modGCDTrace
	}
	var computation error
	_ = tags.EmitConditional("mgcd", "mod_gcd", "src/math/polynomial/upolynomial.cpp", 726, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i, v := range [2]UPolynomialView{a, b} {
			caption := "u: "
			if i != 0 {
				caption = "\nv: "
			}
			_ = linearWriteText(out, caption)
			var issued upolyDisplayInvocationReceipt
			e := m.displayIntoWithReceipt(v, factorizationTraceStream{out}, "x", true, &issued)
			if m.matchesDisplayReceipt(&issued) {
				*slot = issued
				if m.displayReceiptPending(slot) {
					m.pendingModGCDTrace = slot
				}
			}
			if e != nil {
				computation = e
				return e
			}
		}
		_ = linearWriteText(out, "\n")
		return nil
	})
	return computation
}

// Own only the default caller's accepted display, never a frontend or a later
// independent display that reused its frame.
func (m *UPolynomialManager) CleanupModularGCDEntryTrace() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if m.factorTrace != nil && m.factorTrace.executing || m.displayExecuting() {
		return ErrUPolynomialCleanupPending
	}
	return m.CleanupModularGCDTrace()
}

// Source: upolynomial.cpp:772,792,796,806.
func (m *UPolynomialManager) traceModGCDPolynomial(line int, caption string, v UPolynomialView, context integerAllocationInvocationContext) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	slot := &m.modGCDTrace
	if q := context.squareFreeCallFrame(m.config.Integer); q != nil {
		slot = &q.modGCDTrace
	}
	if f := context.factorFrontendFrame(m.config.Integer); f != nil && f.service.manager == m {
		slot = &f.modGCDTrace
	}
	var computation error
	_ = tags.EmitConditional("mgcd", "mod_gcd", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		_ = linearWriteText(out, caption)
		var issued upolyDisplayInvocationReceipt
		e := m.displayIntoWithReceipt(v, factorizationTraceStream{out}, "x", true, &issued)
		if m.matchesDisplayReceipt(&issued) {
			*slot = issued
			if m.displayReceiptPending(slot) {
				m.pendingModGCDTrace = slot
			}
		}
		if e != nil {
			computation = e
			return e
		}
		_ = linearWriteText(out, "\n")
		return nil
	})
	return computation
}
