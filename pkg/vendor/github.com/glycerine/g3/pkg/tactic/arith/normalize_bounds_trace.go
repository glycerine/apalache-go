package arith

// Source: src/tactic/arith/normalize_bounds_tactic.cpp live traces.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *NormalizeBoundsTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return t.bounds.BindTrace(tags)
}
func (t *NormalizeBoundsTactic) emitNormalized(function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("normalize_bounds_tactic", function, "src/tactic/arith/normalize_bounds_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *NormalizeBoundsTactic) traceLower(variable ast.ASTHandle) error {
	return t.emitNormalized("has_lowers", 74, func(out io.Writer) error {
		value, _, found, err := t.bounds.Lower(variable)
		if err != nil {
			return err
		}
		if err = t.bounds.writeExpr(out, variable); err != nil {
			return err
		}
		flag := 0
		if found {
			flag = 1
		}
		_, err = fmt.Fprintf(out, " has_lower: %d val: %s\n", flag, value.RatString())
		return err
	})
}
func (t *NormalizeBoundsTactic) traceNormalized(goal *tactic.Goal) error {
	return t.emitNormalized("operator()", 144, goal.Display)
}
