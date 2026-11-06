package arith

// Source: src/tactic/arith/lia2pb_tactic.cpp live TRACE sites.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *LIA2PBTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return t.bounds.BindTrace(tags)
}

func (t *LIA2PBTactic) emitLIA2PB(function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("lia2pb", function, "src/tactic/arith/lia2pb_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *LIA2PBTactic) traceFailed(term ast.ASTHandle) error {
	return t.emitLIA2PB("throw_failed", 97, func(out io.Writer) error {
		if _, err := io.WriteString(out, "Failed at:\n"); err != nil {
			return err
		}
		if err := t.bounds.writeExpr(out, term); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *LIA2PBTactic) traceDefinition(term ast.ASTHandle, dep ast.ASTDependency) error {
	return t.emitLIA2PB("operator()", 273, func(out io.Writer) error {
		if err := t.bounds.writeExpr(out, term); err != nil {
			return err
		}
		// Dependencies are arena handles in Go; their diagnostic identity replaces
		// the C++ process-specific pointer spelling. Preserve its null spelling.
		if dep.IsNull() {
			_, err := io.WriteString(out, " -> 0\n")
			return err
		}
		_, err := fmt.Fprintf(out, " -> %v\n", dep)
		return err
	})
}
