package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp TRACE sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

// The source ostream keeps its error state without interrupting the tactic.
type nlTraceOstream struct{ out io.Writer }

func (w nlTraceOstream) Write(p []byte) (int, error) { _, _ = w.out.Write(p); return len(p), nil }

func emitNLTrace(m *ast.ASTManager, services model.ModelEvaluatorConfig, function string, line int, body func(io.Writer) error) error {
	tags := services.Trace
	if tags == nil {
		tags = m.BorrowSourceTraceTags()
	}
	if !tags.ShouldTrace("nlsat_smt") {
		return nil
	}
	err := tags.EmitConditional("nlsat_smt", function, "src/tactic/nlsat_smt/nl_purify_tactic.cpp", line, func() bool { return true }, func(out io.Writer) error { return body(nlTraceOstream{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func traceNLMessage(m *ast.ASTManager, services model.ModelEvaluatorConfig, line int, message string) error {
	return emitNLTrace(m, services, "solve", line, func(out io.Writer) error {
		_, err := io.WriteString(out, message)
		return err
	})
}

func traceNLResults(m *ast.ASTManager, services model.ModelEvaluatorConfig, result *tactic.TacticResult) error {
	return emitNLTrace(m, services, "solve", 411, func(out io.Writer) error {
		for _, goal := range result.Goals {
			if _, err := io.WriteString(out, "goal\n"); err != nil {
				return err
			}
			if err := goal.DisplayWithDependencies(out); err != nil {
				return err
			}
		}
		return nil
	})
}
