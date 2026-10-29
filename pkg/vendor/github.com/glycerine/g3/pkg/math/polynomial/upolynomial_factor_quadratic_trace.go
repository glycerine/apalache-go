package polynomial

import "io"

// Source: upolynomial.cpp:2969,3003. Entry precedes coefficient references and
// every quadratic NUM; the factor-pair record precedes DEBUG product creation.
func (s *UPolynomialFactorizationService) traceFactorQuadraticPolynomials(q *upolyFactorFrontendScope, p *upolyVectorState, line int) error {
	m := s.manager
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("factor") {
		return nil
	}
	if q == nil || !q.actual() || !q.active || !q.executing || q.service != s {
		return ErrUPolynomialOwner
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	states := [2]*upolyVectorState{p}
	captions := [2]string{"factor square free (degree == 2):\n"}
	count := 1
	if line == 3003 {
		states = [2]*upolyVectorState{&q.quadratic.f1, &q.quadratic.f2}
		captions = [2]string{"f1: ", "\nf2: "}
		count = 2
	}
	var computation error
	_ = tags.EmitConditional("factor", "factor_2_sqf_pp", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i := 0; i < count; i++ {
			_ = linearWriteText(out, captions[i])
			var issued upolyDisplayInvocationReceipt
			e := m.displayIntoWithReceipt(m.stateView(states[i]), factorizationTraceStream{out}, "x", false, &issued)
			if m.matchesDisplayReceipt(&issued) {
				q.coreTrace = issued
				if m.displayReceiptPending(&q.coreTrace) {
					m.pendingFactorCoreTrace = &q.coreTrace
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
