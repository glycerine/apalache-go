package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp TRACE sites.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *QFNRAPurifyTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}

func (t *QFNRAPurifyTactic) emitTrace(function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = t.services.Trace
	}
	if tags == nil {
		tags = t.manager.BorrowSourceTraceTags()
	}
	if !tags.ShouldTrace("purify_arith") {
		return nil
	}
	err := tags.EmitConditional("purify_arith", function, "src/tactic/arith/purify_arith_tactic.cpp", line, func() bool { return true }, func(out io.Writer) error { return body(fmTraceOstream{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *QFNRAPurifyTactic) traceGoal(goal *tactic.Goal, line int) error {
	return t.emitTrace("operator()", line, goal.Display)
}

func (t *QFNRAPurifyTactic) traceQuantifier(body, rewritten ast.ASTHandle, count int) error {
	return t.emitTrace("process_quantifier", 700, func(out io.Writer) error {
		if _, err := fmt.Fprintf(out, "num_vars: %d\nbody: ", count); err != nil {
			return err
		}
		if err := writeFMExpression(t.manager, t.services, out, body); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\nnew_body: "); err != nil {
			return err
		}
		if err := writeFMExpression(t.manager, t.services, out, rewritten); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (i *qfnraPurifyInvocation) traceConstraints() error {
	return i.owner.emitTrace("operator()", 762, func(out io.Writer) error {
		for _, ref := range i.constraints {
			constraint, err := i.owner.manager.RefGet(ref)
			if err != nil {
				return err
			}
			if _, err = io.WriteString(out, "\n"); err != nil {
				return err
			}
			if err = writeFMExpression(i.owner.manager, i.owner.services, out, constraint); err != nil {
				return err
			}
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
