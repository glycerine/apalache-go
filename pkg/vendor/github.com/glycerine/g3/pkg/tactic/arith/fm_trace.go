package arith

// Source: src/tactic/arith/fm_tactic.cpp live TRACE operands; src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type fmTraceOstream struct{ out io.Writer }

func (w fmTraceOstream) Write(p []byte) (int, error) { _, _ = w.out.Write(p); return len(p), nil }

func emitFMTrace(m *ast.ASTManager, services model.ModelEvaluatorConfig, tag, function string, line int, body func(io.Writer) error) error {
	tags := services.Trace
	if tags == nil {
		tags = m.BorrowSourceTraceTags()
	}
	if !tags.ShouldTrace(tag) {
		return nil
	}
	err := tags.EmitConditional(tag, function, "src/tactic/arith/fm_tactic.cpp", line, func() bool { return true }, func(w io.Writer) error { return body(fmTraceOstream{w}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func writeFMExpression(m *ast.ASTManager, services model.ModelEvaluatorConfig, out io.Writer, expr ast.ASTHandle) error {
	text, err := fmExpressionText(m, services, expr, 0)
	if err != nil {
		return err
	}
	return fmDisplayWrite(out, text)
}
