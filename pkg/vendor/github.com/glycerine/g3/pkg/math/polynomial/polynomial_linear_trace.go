package polynomial

// Source: linear_eq_solver.h:90,124.

import (
	"io"
	"strconv"
)

func (s *RationalLinearSolver) traceIteration(k uint32, line int) error {
	tags := s.integer.BorrowTraceTags()
	if !tags.ShouldTrace("linear_eq_solver") {
		return nil
	}
	var computation error
	_ = tags.EmitConditional("linear_eq_solver", "solve", "src/math/polynomial/linear_eq_solver.h", line,
		func() bool { return true }, func(w io.Writer) error {
			out := linearTraceWriter{w}
			out.AppendCString("iteration ")
			out.AppendUint32(k)
			out.AppendCString("\n")
			computation = s.displaySource(out)
			return computation
		})
	return computation
}
func (s *ModularLinearSolver) traceIteration(k uint32, line int) error {
	tags := s.manager.BorrowTraceTags()
	if !tags.ShouldTrace("linear_eq_solver") {
		return nil
	}
	var computation error
	_ = tags.EmitConditional("linear_eq_solver", "solve", "src/math/polynomial/linear_eq_solver.h", line,
		func() bool { return true }, func(w io.Writer) error {
			out := linearTraceWriter{w}
			out.AppendCString("iteration ")
			out.AppendUint32(k)
			out.AppendCString("\n")
			computation = s.displaySource(out)
			return computation
		})
	return computation
}

// The diagnostic sticky writer retains ostream failure state. Suppress only
// insertion errors here so later numerical/display operands still execute;
// Display computation errors remain separate and stop the enclosing solver.
type linearTraceWriter struct{ io.Writer }

func (w linearTraceWriter) Write(p []byte) (int, error) {
	_, _ = w.Writer.Write(p)
	return len(p), nil
}
func (w linearTraceWriter) WriteString(s string) (int, error) {
	_ = w.AppendCString(s)
	return len(s), nil
}
func (w linearTraceWriter) AppendCString(s string) error {
	if out, ok := w.Writer.(interface{ AppendCString(string) error }); ok {
		_ = out.AppendCString(s)
		return nil
	}
	_, _ = io.WriteString(w.Writer, s)
	return nil
}
func (w linearTraceWriter) AppendInt32(v int32) error {
	if out, ok := w.Writer.(interface{ AppendInt32(int32) error }); ok {
		_ = out.AppendInt32(v)
		return nil
	}
	_, _ = io.WriteString(w.Writer, strconv.FormatInt(int64(v), 10))
	return nil
}
func (w linearTraceWriter) AppendUint32(v uint32) error {
	if out, ok := w.Writer.(interface{ AppendUint32(uint32) error }); ok {
		_ = out.AppendUint32(v)
		return nil
	}
	_, _ = io.WriteString(w.Writer, strconv.FormatUint(uint64(v), 10))
	return nil
}
