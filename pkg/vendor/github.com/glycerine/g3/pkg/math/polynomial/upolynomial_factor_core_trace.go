package polynomial

import (
	"io"
	"strconv"
)

// Source: upolynomial.cpp:3075,3080,3093,3099,3106. All ordinary displays
// belong to the actual admitted frontend and protect its older source suffix.
func (s *UPolynomialFactorizationService) traceFactorCore(q *upolyFactorFrontendScope, line int, j uint32) error {
	tag := "factor_bug"
	if line == 3080 || line == 3093 {
		tag = "factor"
	}
	m := s.manager
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace(tag) {
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
	states := [3]*upolyVectorState{&q.pp, &q.B, &q.A}
	captions := [3]string{"C: ", "\nB: ", "\nA: "}
	count := 3
	switch line {
	case 3080:
		states = [3]*upolyVectorState{&q.A, &q.B}
		captions = [3]string{"A: ", "\nB: "}
		count = 2
	case 3093:
		states = [3]*upolyVectorState{&q.pp}
		captions = [3]string{"const C: "}
		count = 1
	case 3099:
		states = [3]*upolyVectorState{&q.B, &q.D}
		captions = [3]string{"B: ", "\nD: "}
		count = 2
	case 3106:
		states = [3]*upolyVectorState{&q.A}
		captions = [3]string{"A: "}
		count = 1
	}
	var computation error
	_ = tags.EmitConditional(tag, "factor_core", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		if line == 3080 {
			_ = linearWriteText(out, "factor_core main loop j: ")
			_ = linearWriteText(out, strconv.FormatUint(uint64(j), 10))
			_ = linearWriteText(out, "\n")
		}
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
