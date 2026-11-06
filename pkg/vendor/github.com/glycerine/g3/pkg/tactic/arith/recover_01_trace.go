package arith

// Source: src/tactic/arith/recover_01_tactic.cpp TRACE sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func recover01TraceTags(m *ast.ASTManager, trace []*diagnostic.TraceTags, service *diagnostic.TraceTags) *diagnostic.TraceTags {
	if len(trace) != 0 && trace[0] != nil {
		return trace[0]
	}
	if service != nil {
		return service
	}
	return m.BorrowSourceTraceTags()
}

func emitRecover01Trace(m *ast.ASTManager, tags *diagnostic.TraceTags, function string, line int, body func(io.Writer) error) error {
	if !tags.ShouldTrace("recover_01") {
		return nil
	}
	err := tags.EmitConditional("recover_01", function, "src/tactic/arith/recover_01_tactic.cpp", line,
		func() bool { return true }, func(out io.Writer) error { return body(fmTraceOstream{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func emitRecover01Definition(m *ast.ASTManager, trace []*diagnostic.TraceTags, service *diagnostic.TraceTags, variable, definition ast.ASTHandle) error {
	tags := recover01TraceTags(m, trace, service)
	return emitRecover01Trace(m, tags, "process", 288, func(out io.Writer) error {
		if err := ast.DisplayLegacySMT(out, m, variable, 0, nil); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " --> "); err != nil {
			return err
		}
		if err := ast.DisplayLegacySMT(out, m, definition, 0, nil); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func emitRecover01Goal(goal *tactic.Goal, trace []*diagnostic.TraceTags, service *diagnostic.TraceTags) error {
	tags := recover01TraceTags(goal.Manager(), trace, service)
	return emitRecover01Trace(goal.Manager(), tags, "operator()", 376, goal.Display)
}
