package polynomial

// Source: upolynomial.cpp:532,535,551; retained root-ID/public Sturm traces.

import (
	"bytes"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

type upolySturmRemainderTrace struct {
	displays   [2]upolyDisplayInvocationReceipt
	insertions [2]unumeral.NumeralInsertionReceipt
}

// Stream insertion failures set the diagnostic ostream failbit. Numerical
// display/to_string computations still execute at their source positions.
type upolyRemainderTraceStream struct{ io.Writer }

func (s upolyRemainderTraceStream) AppendCString(v string) error {
	_, _ = io.WriteString(s.Writer, v)
	return nil
}
func (s upolyRemainderTraceStream) AppendNumeralText(v string) error { return s.AppendCString(v) }
func (s upolyRemainderTraceStream) AppendInt32(v int32) error {
	return s.AppendCString(strconv.FormatInt(int64(v), 10))
}

func (r *upolySturmRemainderTrace) pending(m *UPolynomialManager) bool {
	for i := range r.displays {
		if m.displayReceiptPending(&r.displays[i]) {
			return true
		}
	}
	for i := range r.insertions {
		if r.insertions[i].Active() {
			return true
		}
	}
	return false
}
func (r *upolySturmRemainderTrace) sourceCause(m *UPolynomialManager) error {
	for i := range r.displays {
		if e := m.displayReceiptSourceCause(&r.displays[i]); e != nil {
			return e
		}
	}
	for i := range r.insertions {
		if e := r.insertions[i].SourceCause(); e != nil {
			return e
		}
	}
	return nil
}
func (r *upolySturmRemainderTrace) cleanup(m *UPolynomialManager, abort bool) error {
	z := m.config.Integer
	for i := len(r.insertions) - 1; i >= 0; i-- {
		child := &r.insertions[i]
		if child.Active() {
			if e := child.Complete(z.Naturals(), abort); e != nil {
				return e
			}
		}
	}
	if !abort {
		for i := len(r.displays) - 1; i >= 0; i-- {
			if e := m.cleanupDisplayReceipt(&r.displays[i]); e != nil {
				return e
			}
		}
	}
	return nil
}

func (q *upolyRootIDScope) remainderTracePending() bool { return q.remainderTrace.pending(q.owner) }
func (q *upolyRootIDScope) remainderTraceSourceCause() error {
	return q.remainderTrace.sourceCause(q.owner)
}
func (q *upolyRootIDScope) cleanupRemainderTrace(abort bool) error {
	return q.remainderTrace.cleanup(q.owner, abort)
}
func (q *upolyRootIDScope) traceRemainder(phase uint8, b UPolynomialView, r *upolyVectorState, bn *unumeral.Int) error {
	m := q.owner
	if !m.config.Integer.BorrowTraceTags().ShouldTrace("rem_bug") {
		return nil
	}
	if q.context().rootIDFrame(m.config.Integer) != q || !q.scalar.constructed {
		return ErrUPolynomialOwner
	}
	if q.remainderTracePending() {
		return ErrUPolynomialCleanupPending
	}
	return q.remainderTrace.emit(m, phase, b, r, bn, &q.scalar.value)
}

func (trace *upolySturmRemainderTrace) emit(m *UPolynomialManager, phase uint8, b UPolynomialView, r *upolyVectorState, bn, scalar *unumeral.Int) error {
	tags := m.config.Integer.BorrowTraceTags()
	line := 532
	if phase == 1 {
		line = 535
	} else if phase == 2 {
		line = 551
	}
	var computation error
	m.callbackDepth++
	defer func() { m.callbackDepth-- }()
	_ = tags.EmitConditional("rem_bug", "rem", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		var e error
		switch phase {
		case 0:
			_ = out.AppendCString("rem loop, p2:\n")
			e = m.displayIntoWithReceipt(b, out, "x", false, &trace.displays[0])
			if e == nil {
				_ = out.AppendCString("\nbuffer:\n")
				e = m.displayIntoWithReceipt(m.stateView(r), out, "x", false, &trace.displays[1])
			}
			if e == nil {
				_ = out.AppendCString("\n")
			}
		case 1:
			_ = out.AppendCString("finished\n")
		case 2:
			for i, a := range [2]*unumeral.Int{scalar, bn} {
				if i == 0 {
					_ = out.AppendCString("a_m: ")
				} else {
					_ = out.AppendCString(", b_n: ")
				}
				var text bytes.Buffer
				e = m.config.Integer.InsertSourceIntegerTracked(upolyNormalizeStringStream{&text}, a, &trace.insertions[i], nil)
				if e != nil {
					break
				}
				_ = out.AppendCString(text.String())
			}
			if e == nil {
				_ = out.AppendCString("\n")
			}
		}
		computation = e
		return e
	})
	return computation
}
