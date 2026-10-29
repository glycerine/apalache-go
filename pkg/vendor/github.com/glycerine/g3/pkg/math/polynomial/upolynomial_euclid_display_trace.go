package polynomial

import "io"

// Source: upolynomial.cpp:836,847-848. Ordinary display uses spaces, not stars.
func (m *UPolynomialManager) traceEuclidPolynomials(line int, a, b UPolynomialView, result *upolyVectorState, context integerAllocationInvocationContext) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	slot := &m.euclidTrace
	if q := context.squareFreeCallFrame(m.config.Integer); q != nil {
		slot = &q.euclidTrace
	}
	if f := context.factorFrontendFrame(m.config.Integer); f != nil && f.service.manager == m {
		slot = &f.euclidTrace
	}
	views := [3]UPolynomialView{m.stateView(&m.gcd1), m.stateView(&m.gcd2)}
	captions := [3]string{"A: ", "\nB: "}
	count := 2
	if line == 847 {
		views = [3]UPolynomialView{a, b, m.stateView(result)}
		captions = [3]string{"GCD\n", "\n", "\n--->\n"}
		count = 3
	}
	var computation error
	_ = tags.EmitConditional("upolynomial", "euclid_gcd", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i := 0; i < count; i++ {
			_ = linearWriteText(out, captions[i])
			var issued upolyDisplayInvocationReceipt
			e := m.displayIntoWithReceipt(views[i], factorizationTraceStream{out}, "x", false, &issued)
			if m.matchesDisplayReceipt(&issued) {
				*slot = issued
				if m.displayReceiptPending(slot) {
					m.pendingEuclidTrace = slot
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
func (m *UPolynomialManager) CleanupEuclideanGCDTrace() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if m.factorTrace != nil && m.factorTrace.executing || m.displayExecuting() {
		return ErrUPolynomialCleanupPending
	}
	return m.euclidTrace.cleanupOwned()
}
