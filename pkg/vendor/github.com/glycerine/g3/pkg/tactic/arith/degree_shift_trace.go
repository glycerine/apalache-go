package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp TRACE sites.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *DegreeShiftTactic) traceTags() *diagnostic.TraceTags {
	if t.trace != nil {
		return t.trace
	}
	if t.services.Trace != nil {
		return t.services.Trace
	}
	return t.manager.BorrowSourceTraceTags()
}

func (t *DegreeShiftTactic) emitTrace(function string, line int, body func(io.Writer) error) error {
	tags := t.traceTags()
	if !tags.ShouldTrace("degree_shift") {
		return nil
	}
	err := tags.EmitConditional("degree_shift", function, "src/tactic/arith/degree_shift_tactic.cpp", line,
		func() bool { return true }, func(out io.Writer) error { return body(fmTraceOstream{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *DegreeShiftTactic) traceCandidates(plan *degreeShiftPlan) error {
	return t.emitTrace("collect", 186, func(out io.Writer) error {
		if _, err := io.WriteString(out, "candidates:\n"); err != nil {
			return err
		}
		for _, entry := range plan.entries {
			original, err := t.manager.RefGet(entry.original)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, "POWER: %s\n", entry.degree); err != nil {
				return err
			}
			if err = ast.DisplayLegacySMT(out, t.manager, original, 0, nil); err != nil {
				return err
			}
			if _, err = io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}

func (t *DegreeShiftTactic) traceResult(goal *tactic.Goal, result *tactic.TacticResult) error {
	return t.emitTrace("operator()", 289, func(out io.Writer) error {
		if err := goal.Display(out); err != nil {
			return err
		}
		if result.ModelConverter != nil {
			return result.ModelConverter.Display(out)
		}
		return nil
	})
}
