package polynomial

// Source: upolynomial.cpp:2401,2455-2461; retained isolation diagnostics.

import (
	"bytes"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

func (q *upolySturmIsolationScope) tracePending() bool {
	for i := range q.traceDisplays {
		if q.owner.displayReceiptPending(&q.traceDisplays[i]) {
			return true
		}
	}
	for i := range q.traceInsertions {
		if q.traceInsertions[i].Active() {
			return true
		}
	}
	return false
}
func (q *upolySturmIsolationScope) traceSourceCause() error {
	for i := range q.traceDisplays {
		if e := q.owner.displayReceiptSourceCause(&q.traceDisplays[i]); e != nil {
			return e
		}
	}
	for i := range q.traceInsertions {
		if e := q.traceInsertions[i].SourceCause(); e != nil {
			return e
		}
	}
	return nil
}
func (q *upolySturmIsolationScope) cleanupTrace(abort bool) error {
	for i := len(q.traceInsertions) - 1; i >= 0; i-- {
		r := &q.traceInsertions[i]
		if r.Active() {
			if e := r.Complete(q.owner.config.Integer.Naturals(), abort); e != nil {
				return e
			}
		}
	}
	if !abort {
		for i := len(q.traceDisplays) - 1; i >= 0; i-- {
			if e := q.owner.cleanupDisplayReceipt(&q.traceDisplays[i]); e != nil {
				return e
			}
		}
	}
	return nil
}
func (q *upolySturmIsolationScope) traceInitial(v UPolynomialView) error {
	m := q.owner
	tags := m.config.Integer.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	if !q.entryValid() || !q.live[4] {
		return ErrUPolynomialOwner
	}
	if q.tracePending() {
		return ErrUPolynomialCleanupPending
	}
	var computation error
	m.callbackDepth++
	defer func() { m.callbackDepth-- }()
	_ = tags.EmitConditional("upolynomial", "sturm_isolate_roots_core", "src/math/polynomial/upolynomial.cpp", 2401, func() bool { return true }, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		_ = out.AppendCString("p: ")
		computation = m.displayIntoWithReceipt(v, out, "x", false, &q.traceDisplays[0])
		if computation != nil {
			return computation
		}
		_ = out.AppendCString("\nSturm seq:\n")
		computation = m.displaySequenceIntoWithReceipt(&q.sequence, out, "x", &q.traceDisplays[1])
		if computation == nil {
			_ = out.AppendCString("\n")
		}
		return computation
	})
	return computation
}
func (q *upolySturmIsolationScope) traceBinary(w io.Writer, a *unumeral.BinaryRational, i int) error {
	var text bytes.Buffer
	if e := q.owner.config.Integer.InsertSourceIntegerTracked(upolyNormalizeStringStream{&text}, &a.Num, &q.traceInsertions[i], nil); e != nil {
		return e
	}
	_, _ = io.WriteString(w, text.String())
	if a.KField != 0 {
		_, _ = io.WriteString(w, "/2")
		if a.KField != 1 {
			_, _ = io.WriteString(w, "^"+strconv.FormatUint(uint64(a.KField), 10))
		}
	}
	return nil
}
func (q *upolySturmIsolationScope) traceMidpoint(lower, upper uint32) error {
	m := q.owner
	tags := m.config.Integer.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	if !q.entryValid() || !q.live[4] {
		return ErrUPolynomialOwner
	}
	if q.tracePending() {
		return ErrUPolynomialCleanupPending
	}
	var computation error
	m.callbackDepth++
	defer func() { m.callbackDepth-- }()
	_ = tags.EmitConditional("upolynomial", "sturm_isolate_roots_core", "src/math/polynomial/upolynomial.cpp", 2455, func() bool { return true }, func(w io.Writer) error {
		_, _ = io.WriteString(w, "depth: "+strconv.Itoa(len(q.stack.values))+"\nlower_sv: "+strconv.FormatUint(uint64(lower), 10)+"\nupper_sv: "+strconv.FormatUint(uint64(upper), 10)+"\n")
		for i, a := range [3]*unumeral.BinaryRational{&q.lower, &q.upper, &q.mid} {
			_, _ = io.WriteString(w, [3]string{"curr_lower: ", "curr_upper: ", "mid: "}[i])
			computation = q.traceBinary(w, a, i)
			if computation != nil {
				return computation
			}
			_, _ = io.WriteString(w, "\n")
		}
		return nil
	})
	return computation
}
