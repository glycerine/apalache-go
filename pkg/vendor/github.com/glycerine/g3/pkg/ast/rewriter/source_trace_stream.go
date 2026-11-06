package rewriter

// Source: src/util/trace.h. A failed ostream insertion changes stream state,
// while subsequent operands and statements in the trace body still execute.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type rewriterTraceStream struct{ out io.Writer }

// The emitter must distinguish an operand exception from badbit even if both
// use the same error value. This private marker never escapes this adapter.
type rewriterTraceOperandFailure struct{ cause error }

func (e *rewriterTraceOperandFailure) Error() string { return e.cause.Error() }

func (w rewriterTraceStream) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	return len(p), nil
}

func (w rewriterTraceStream) WriteString(s string) (int, error) {
	_, _ = io.WriteString(w.out, s)
	return len(s), nil
}

func (w rewriterTraceStream) Flush() error {
	if out, ok := w.out.(interface{ Flush() error }); ok {
		_ = out.Flush()
	}
	return nil
}

func emitRewriterSourceTrace(tags *diagnostic.TraceTags, tag, function, file string, line int, body func(io.Writer) error) error {
	var operandErr error
	err := tags.EmitConditional(tag, function, file, line, func() bool { return true }, func(out io.Writer) error {
		operandErr = body(rewriterTraceStream{out})
		if operandErr != nil {
			return &rewriterTraceOperandFailure{operandErr}
		}
		return nil
	})
	// Keep operand failures distinct even when their error identity happens to
	// match the stream's earlier error. Only insertion failures are nonthrowing.
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
